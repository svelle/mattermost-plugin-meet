package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-meet/server/store/kvstore"
)

// mockPluginAPI implements the plugin.API interface methods we need for testing.
type mockPluginAPI struct {
	plugin.API
	siteURL string
	user    *model.User
	userErr *model.AppError
	post    *model.Post
	postErr *model.AppError
	logged  []string
	hasPerm bool
}

func (m *mockPluginAPI) GetConfig() *model.Config {
	return &model.Config{
		ServiceSettings: model.ServiceSettings{
			SiteURL: &m.siteURL,
		},
	}
}

func (m *mockPluginAPI) LogDebug(msg string, keyValuePairs ...any)  {}
func (m *mockPluginAPI) LogInfo(msg string, keyValuePairs ...any)   {}
func (m *mockPluginAPI) LogWarn(msg string, keyValuePairs ...any)   {}
func (m *mockPluginAPI) LogError(msg string, keyValuePairs ...any)  { m.logged = append(m.logged, msg) }
func (m *mockPluginAPI) LoadPluginConfiguration(_ any) error {
	return nil
}

func (m *mockPluginAPI) GetUser(userID string) (*model.User, *model.AppError) {
	if m.userErr != nil {
		return nil, m.userErr
	}
	if m.user != nil {
		return m.user, nil
	}
	return &model.User{}, nil
}

func (m *mockPluginAPI) CreatePost(post *model.Post) (*model.Post, *model.AppError) {
	m.post = post
	if m.postErr != nil {
		return nil, m.postErr
	}
	return post, nil
}

func (m *mockPluginAPI) HasPermissionToChannel(userID, channelID string, permission *model.Permission) bool {
	return m.hasPerm
}

// mockKVStore implements kvstore.KVStore for testing.
type mockKVStore struct {
	tokens map[string]*kvstore.OAuth2Token
	states map[string]string
	err    error
}

func newMockKVStore() *mockKVStore {
	return &mockKVStore{
		tokens: make(map[string]*kvstore.OAuth2Token),
		states: make(map[string]string),
	}
}

func (m *mockKVStore) StoreOAuth2Token(userID string, token *kvstore.OAuth2Token) error {
	if m.err != nil {
		return m.err
	}
	m.tokens[userID] = token
	return nil
}

func (m *mockKVStore) GetOAuth2Token(userID string) (*kvstore.OAuth2Token, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tokens[userID], nil
}

func (m *mockKVStore) DeleteOAuth2Token(userID string) error {
	delete(m.tokens, userID)
	return nil
}

func (m *mockKVStore) StoreOAuth2State(state string, userID string) error {
	if m.err != nil {
		return m.err
	}
	m.states[state] = userID
	return nil
}

func (m *mockKVStore) GetAndDeleteOAuth2State(state string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	userID, ok := m.states[state]
	if !ok {
		return "", kvstore.ErrStateNotFound
	}
	delete(m.states, state)
	return userID, nil
}

func setupPlugin(t *testing.T) (*Plugin, *mockPluginAPI, *mockKVStore) {
	t.Helper()

	api := &mockPluginAPI{
		siteURL: "http://localhost:8065",
		hasPerm: true,
	}
	kv := newMockKVStore()

	p := &Plugin{}
	p.MattermostPlugin.API = api
	p.kvstore = kv
	p.setConfiguration(&configuration{
		GoogleClientID:     "test-client-id",
		GoogleClientSecret: "test-client-secret",
		EncryptionKey:      "test-encryption-key",
	})
	p.router = p.initRouter()

	return p, api, kv
}

func TestMattermostAuthorizationRequired(t *testing.T) {
	p, _, _ := setupPlugin(t)

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/meeting", nil)
		w := httptest.NewRecorder()
		p.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("allows authenticated request", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"channel_id": "chan1"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/meeting", bytes.NewReader(body))
		req.Header.Set("Mattermost-User-ID", "user1")
		w := httptest.NewRecorder()
		p.router.ServeHTTP(w, req)
		// Should not be 401 — it may be another status depending on state, but not unauthorized
		assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	})
}

func TestHandleConfigStatus(t *testing.T) {
	t.Run("configured plugin", func(t *testing.T) {
		p, api, _ := setupPlugin(t)
		api.user = &model.User{Roles: "system_user"}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/config/status", nil)
		req.Header.Set("Mattermost-User-ID", "user1")
		w := httptest.NewRecorder()
		p.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, true, resp["configured"])
	})

	t.Run("unconfigured plugin with admin", func(t *testing.T) {
		p, api, _ := setupPlugin(t)
		api.user = &model.User{Roles: "system_admin system_user"}
		p.setConfiguration(&configuration{}) // empty = unconfigured

		req := httptest.NewRequest(http.MethodGet, "/api/v1/config/status", nil)
		req.Header.Set("Mattermost-User-ID", "admin1")
		w := httptest.NewRecorder()
		p.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, false, resp["configured"])
		assert.Contains(t, resp["configure_url"], "admin_console")
	})
}

func TestHandleCreateMeeting_NotConfigured(t *testing.T) {
	p, api, _ := setupPlugin(t)
	api.user = &model.User{Roles: "system_admin system_user"}
	p.setConfiguration(&configuration{}) // empty

	body, _ := json.Marshal(map[string]string{"channel_id": "chan1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/meeting", bytes.NewReader(body))
	req.Header.Set("Mattermost-User-ID", "admin1")
	w := httptest.NewRecorder()
	p.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "not_configured", resp["error"])
}

func TestHandleCreateMeeting_BadRequest(t *testing.T) {
	p, _, _ := setupPlugin(t)

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/meeting", bytes.NewReader([]byte("not json")))
		req.Header.Set("Mattermost-User-ID", "user1")
		w := httptest.NewRecorder()
		p.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]string
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp["error"], "Invalid request body")
	})

	t.Run("missing channel_id", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"topic": "test"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/meeting", bytes.NewReader(body))
		req.Header.Set("Mattermost-User-ID", "user1")
		w := httptest.NewRecorder()
		p.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]string
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp["error"], "channel_id is required")
	})
}

func TestHandleCreateMeeting_NotConnected(t *testing.T) {
	p, _, _ := setupPlugin(t)
	// No token stored — user is not connected

	body, _ := json.Marshal(map[string]string{"channel_id": "chan1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/meeting", bytes.NewReader(body))
	req.Header.Set("Mattermost-User-ID", "user1")
	w := httptest.NewRecorder()
	p.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "not_connected", resp["error"])
	assert.Contains(t, resp["connect_url"], "oauth/connect")
}

func TestHandleCreateMeeting_Success(t *testing.T) {
	// Create a mock Meet API server
	meetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := meetSpaceResponse{
			Name:        "spaces/test",
			MeetingURI:  "https://meet.google.com/test-meet",
			MeetingCode: "test-meet",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer meetServer.Close()

	origURL := googleMeetURL
	origClient := httpClient
	googleMeetURL = meetServer.URL + "/v2"
	httpClient = meetServer.Client()
	defer func() {
		googleMeetURL = origURL
		httpClient = origClient
	}()

	p, api, kv := setupPlugin(t)
	api.hasPerm = true

	// Store a valid token
	kv.tokens["user1"] = &kvstore.OAuth2Token{
		AccessToken:  "valid-token",
		TokenType:    "Bearer",
		RefreshToken: "refresh",
		Expiry:       time.Now().Add(time.Hour),
	}

	body, _ := json.Marshal(map[string]string{"channel_id": "chan1", "topic": "Standup"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/meeting", bytes.NewReader(body))
	req.Header.Set("Mattermost-User-ID", "user1")
	w := httptest.NewRecorder()
	p.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp["status"])

	// Verify the post was created
	require.NotNil(t, api.post)
	assert.Equal(t, "chan1", api.post.ChannelId)
	assert.Equal(t, "custom_google_meet", api.post.Type)
	assert.Equal(t, "https://meet.google.com/test-meet", api.post.Props["meeting_link"])
}

func TestHandleErrorWithCode(t *testing.T) {
	p, api, _ := setupPlugin(t)

	w := httptest.NewRecorder()
	p.handleErrorWithCode(w, http.StatusBadRequest, "Something went wrong.", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Something went wrong.", resp["error"])

	// Verify error was logged
	require.Len(t, api.logged, 1)
	assert.Contains(t, api.logged[0], "Something went wrong.")
}

func TestHandleOAuthCallback_MissingParams(t *testing.T) {
	p, _, _ := setupPlugin(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/oauth/callback", nil)
	w := httptest.NewRecorder()
	p.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGenerateState(t *testing.T) {
	state1, err := generateState()
	require.NoError(t, err)
	assert.Len(t, state1, 32, "state should be 32 hex characters (16 bytes)")

	state2, err := generateState()
	require.NoError(t, err)
	assert.NotEqual(t, state1, state2, "states should be unique")
}
