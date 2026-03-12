package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/plugin"
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

func (p *Plugin) handleOAuthConnect(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")

	config := p.getConfiguration()
	if err := config.IsValid(); err != nil {
		http.Error(w, "Plugin is not configured. Please contact your system administrator.", http.StatusInternalServerError)
		return
	}

	state, err := generateState()
	if err != nil {
		http.Error(w, "Failed to generate state", http.StatusInternalServerError)
		return
	}

	if err := p.kvstore.StoreOAuth2State(state, userID); err != nil {
		http.Error(w, "Failed to store state", http.StatusInternalServerError)
		return
	}

	authURL := p.buildAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (p *Plugin) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		http.Error(w, "Missing code or state parameter", http.StatusBadRequest)
		return
	}

	userID, err := p.kvstore.GetAndDeleteOAuth2State(state)
	if err != nil {
		http.Error(w, "Invalid or expired state. Please try connecting again.", http.StatusBadRequest)
		return
	}

	token, err := p.exchangeCodeForToken(code)
	if err != nil {
		p.API.LogError("Failed to exchange code for token", "error", err.Error())
		http.Error(w, "Failed to complete authentication. Please try again.", http.StatusInternalServerError)
		return
	}

	if err := p.kvstore.StoreOAuth2Token(userID, token); err != nil {
		p.API.LogError("Failed to store token", "error", err.Error())
		http.Error(w, "Failed to save authentication. Please try again.", http.StatusInternalServerError)
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

	isAdmin, _ := p.IsUserAdmin(userID)
	configured := p.IsPluginConfigured()

	resp := map[string]interface{}{
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
	p.API.LogDebug("handleCreateMeeting called", "user_id", userID)

	if !p.IsPluginConfigured() {
		p.API.LogDebug("Plugin not configured")
		isAdmin, _ := p.IsUserAdmin(userID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		resp := map[string]string{"error": "bad_request", "message": "Invalid request body"}
		json.NewEncoder(w).Encode(resp)
		return
	}

	if req.ChannelID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		resp := map[string]string{"error": "bad_request", "message": "channel_id is required"}
		json.NewEncoder(w).Encode(resp)
		return
	}

	p.API.LogDebug("Checking user connection status", "user_id", userID)
	connected, err := p.IsUserConnected(userID)
	if err != nil {
		p.API.LogError("Failed to check connection status", "user_id", userID, "error", err.Error())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]string{"error": "meeting_failed", "message": "Failed to check connection status"}
		json.NewEncoder(w).Encode(resp)
		return
	}

	if !connected {
		p.API.LogDebug("User not connected to Google", "user_id", userID)
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

	p.API.LogDebug("Starting meeting", "user_id", userID, "channel_id", req.ChannelID, "topic", req.Topic)
	if err := p.StartMeeting(userID, req.ChannelID, req.Topic); err != nil {
		p.API.LogError("Failed to create meeting", "user_id", userID, "channel_id", req.ChannelID, "error", err.Error())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]string{
			"error":   "meeting_failed",
			"message": "Failed to create meeting. Please try again or check the server logs.",
		}
		if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
			p.API.LogError("Failed to encode response", "error", encErr.Error())
		}
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
