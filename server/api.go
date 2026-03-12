package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/mattermost/mattermost-plugin-meet/server/pluginerrors"
)

func (p *Plugin) initRouter() *mux.Router {
	router := mux.NewRouter()

	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	// OAuth callback does NOT require Mattermost auth (Google redirects here)
	apiRouter.HandleFunc("/oauth/callback", p.handleOAuthCallback).Methods(http.MethodGet)

	// Protected routes require Mattermost auth
	protectedRouter := apiRouter.PathPrefix("").Subrouter()
	protectedRouter.Use(p.MattermostAuthorizationRequired)
	protectedRouter.HandleFunc("/oauth/connect", p.handleOAuthConnect).Methods(http.MethodGet)
	protectedRouter.HandleFunc("/meeting", p.handleCreateMeeting).Methods(http.MethodPost)
	protectedRouter.HandleFunc("/config/status", p.handleConfigStatus).Methods(http.MethodGet)

	return router
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

func (p *Plugin) MattermostAuthorizationRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("Mattermost-User-ID")
		if userID == "" {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleError logs the internal error and sends a generic 500 JSON response.
func (p *Plugin) handleError(w http.ResponseWriter, internalErr error) {
	p.handleErrorWithCode(w, http.StatusInternalServerError, "An internal error has occurred. Check app server logs for details.", internalErr)
}

// handleErrorWithCode logs the internal error and sends the public facing error
// message as JSON in a response with the provided code.
func (p *Plugin) handleErrorWithCode(w http.ResponseWriter, code int, publicErrorMsg string, internalErr error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	details := ""
	if internalErr != nil {
		details = internalErr.Error()
	}

	p.API.LogError(fmt.Sprintf("public error message: %v; internal details: %v", publicErrorMsg, details))

	responseMsg, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{
		Error: publicErrorMsg,
	})
	_, _ = w.Write(responseMsg)
}

func (p *Plugin) handleOAuthConnect(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")

	config := p.getConfiguration()
	if err := config.IsValid(); err != nil {
		p.handleError(w, err)
		return
	}

	state, err := generateState()
	if err != nil {
		p.handleError(w, fmt.Errorf("failed to generate state: %w", err))
		return
	}

	if err := p.kvstore.StoreOAuth2State(state, userID); err != nil {
		p.handleError(w, fmt.Errorf("failed to store state: %w", err))
		return
	}

	authURL := p.buildAuthURL(state)
	if authURL == "" {
		p.handleError(w, errors.New("failed to build OAuth URL: Mattermost Site URL is not configured"))
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (p *Plugin) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		p.handleErrorWithCode(w, http.StatusBadRequest, "Missing code or state parameter.", nil)
		return
	}

	userID, err := p.kvstore.GetAndDeleteOAuth2State(state)
	if err != nil {
		p.handleErrorWithCode(w, http.StatusBadRequest, "Invalid or expired state. Please try connecting again.", err)
		return
	}

	token, err := p.exchangeCodeForToken(code)
	if err != nil {
		p.handleError(w, fmt.Errorf("failed to exchange code for token: %w", err))
		return
	}

	if err := p.kvstore.StoreOAuth2Token(userID, token); err != nil {
		p.handleError(w, fmt.Errorf("failed to store token: %w", err))
		return
	}

	html := `
<!DOCTYPE html>
<html>
<head><title>Google Meet - Connected</title></head>
<body>
<h2>Successfully connected to Google!</h2>
<p>You can close this window and return to Mattermost. Use <code>/meet</code> to start a meeting.</p>
<script>
setTimeout(function() { window.close(); }, 3000);
</script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(html)); err != nil {
		p.API.LogError("Failed to write response", "error", err.Error())
	}
}

type createMeetingRequest struct {
	ChannelID string `json:"channel_id"`
	Topic     string `json:"topic"`
}

func (p *Plugin) handleConfigStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")

	isAdmin, err := p.IsUserAdmin(userID)
	if err != nil {
		p.API.LogError("IsUserAdmin failed", "user_id", userID, "error", err.Error())
		http.Error(w, "Failed to determine admin status.", http.StatusInternalServerError)
		return
	}
	configured := p.IsPluginConfigured()

	resp := map[string]any{
		"configured": configured,
		"is_admin":   isAdmin,
	}
	if isAdmin && !configured {
		resp["configure_url"] = p.GetPluginConfigureURL()
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		p.API.LogError("Failed to encode response", "error", err.Error())
	}
}

func (p *Plugin) handleCreateMeeting(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")

	if !p.IsPluginConfigured() {
		isAdmin, err := p.IsUserAdmin(userID)
		if err != nil {
			p.API.LogError("IsUserAdmin failed", "user_id", userID, "error", err.Error())
			http.Error(w, "Failed to determine admin status.", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"error":    "not_configured",
			"is_admin": isAdmin,
		}
		if isAdmin {
			resp["configure_url"] = p.GetPluginConfigureURL()
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			p.API.LogError("Failed to encode response", "error", err.Error())
		}
		return
	}

	var req createMeetingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.handleErrorWithCode(w, http.StatusBadRequest, "Invalid request body.", err)
		return
	}

	if req.ChannelID == "" {
		p.handleErrorWithCode(w, http.StatusBadRequest, "channel_id is required.", nil)
		return
	}

	connected, err := p.IsUserConnected(userID)
	if err != nil {
		p.handleError(w, fmt.Errorf("failed to check connection status: %w", err))
		return
	}

	if !connected {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]string{
			"error":       "not_connected",
			"connect_url": p.GetConnectURL(),
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			p.API.LogError("Failed to encode response", "error", err.Error())
		}
		return
	}

	if err := p.StartMeeting(userID, req.ChannelID, req.Topic); err != nil {
		if errors.Is(err, pluginerrors.ErrNeedsReconnect) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp := map[string]string{
				"error":       "not_connected",
				"connect_url": p.GetConnectURL(),
			}
			if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
				p.API.LogError("Failed to encode response", "error", encErr.Error())
			}
			return
		}
		p.handleError(w, fmt.Errorf("failed to create meeting: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]string{"status": "ok"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		p.API.LogError("Failed to encode response", "error", err.Error())
	}
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
