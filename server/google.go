package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mattermost/mattermost-plugin-google-meet/server/store/kvstore"
)

// ErrInsufficientScopes indicates the token lacks the required OAuth scope.
var ErrInsufficientScopes = errors.New("insufficient authentication scopes")

const (
	googleAuthURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	meetScope          = "https://www.googleapis.com/auth/meetings.space.created"
	tokenRefreshBuffer = 5 * time.Minute
)

// These are vars so tests can override them with httptest servers.
var (
	// #nosec G101 -- OAuth endpoint URLs are public constants, not credentials.
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleMeetURL  = "https://meet.googleapis.com/v2"
	httpClient     = &http.Client{Timeout: 30 * time.Second}
)

func (p *Plugin) getOAuth2CallbackURL() string {
	siteURL := p.getSiteURL()
	if siteURL == "" {
		return ""
	}

	return fmt.Sprintf("%s/plugins/%s/api/v1/oauth/callback", siteURL, manifestID())
}

func (p *Plugin) getOAuth2ConnectURL() string {
	siteURL := p.getSiteURL()
	if siteURL == "" {
		return ""
	}

	return fmt.Sprintf("%s/plugins/%s/api/v1/oauth/connect", siteURL, manifestID())
}

func (p *Plugin) buildAuthURL(state string) string {
	config := p.getConfiguration()
	redirectURI := p.getOAuth2CallbackURL()
	if redirectURI == "" {
		return ""
	}

	params := url.Values{
		"client_id":     {config.GoogleClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {meetScope},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
		"state":         {state},
	}

	return googleAuthURL + "?" + params.Encode()
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func (p *Plugin) exchangeCodeForToken(code string) (*kvstore.OAuth2Token, error) {
	config := p.getConfiguration()
	redirectURI := p.getOAuth2CallbackURL()
	if redirectURI == "" {
		return nil, errors.New("mattermost site URL is not configured")
	}

	data := url.Values{
		"code":          {code},
		"client_id":     {config.GoogleClientID},
		"client_secret": {config.GoogleClientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}

	resp, err := httpClient.PostForm(googleTokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && p.API != nil {
			p.API.LogWarn("Failed to close response body", "description", "token exchange response", "error", closeErr.Error())
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	token := &kvstore.OAuth2Token{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	return token, nil
}

func (p *Plugin) refreshToken(token *kvstore.OAuth2Token) (*kvstore.OAuth2Token, error) {
	config := p.getConfiguration()

	data := url.Values{
		"client_id":     {config.GoogleClientID},
		"client_secret": {config.GoogleClientSecret},
		"refresh_token": {token.RefreshToken},
		"grant_type":    {"refresh_token"},
	}

	resp, err := httpClient.PostForm(googleTokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && p.API != nil {
			p.API.LogWarn("Failed to close response body", "description", "token refresh response", "error", closeErr.Error())
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	newToken := &kvstore.OAuth2Token{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: token.RefreshToken, // keep existing refresh token
		Expiry:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	if tokenResp.RefreshToken != "" {
		newToken.RefreshToken = tokenResp.RefreshToken
	}

	return newToken, nil
}

func (p *Plugin) getValidToken(userID string) (*kvstore.OAuth2Token, error) {
	store, err := p.getOAuthKVStore()
	if err != nil {
		return nil, err
	}

	token, err := store.GetOAuth2Token(userID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, nil
	}

	// Refresh early to avoid edge-of-expiry failures in slower environments.
	if time.Until(token.Expiry) < tokenRefreshBuffer {
		token, err = p.refreshToken(token)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh expired token: %w", err)
		}
		if err := store.StoreOAuth2Token(userID, token); err != nil {
			p.API.LogError("Failed to store refreshed token", "error", err.Error())
		}
	}

	return token, nil
}

type meetSpaceResponse struct {
	Name        string `json:"name"`
	MeetingURI  string `json:"meetingUri"`
	MeetingCode string `json:"meetingCode"`
}

func (p *Plugin) createMeeting(token *kvstore.OAuth2Token, _ string) (string, error) {
	reqURL := googleMeetURL + "/spaces"
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader("{}"))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create meeting space: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && p.API != nil {
			p.API.LogWarn("Failed to close response body", "description", "meeting creation response", "error", closeErr.Error())
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusForbidden && strings.Contains(string(body), "ACCESS_TOKEN_SCOPE_INSUFFICIENT") {
		return "", ErrInsufficientScopes
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("meet API returned status %d: %s", resp.StatusCode, string(body))
	}

	var space meetSpaceResponse
	if err := json.Unmarshal(body, &space); err != nil {
		return "", fmt.Errorf("failed to parse Meet API response: %w", err)
	}

	if space.MeetingURI == "" {
		return "", errors.New("no meeting URI in Meet API response")
	}

	return space.MeetingURI, nil
}
