package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-meet/server/store/kvstore"
)

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleCalendarURL = "https://www.googleapis.com/calendar/v3"
	calendarScope     = "https://www.googleapis.com/auth/calendar.events"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

func (p *Plugin) getOAuth2ConnectURL() string {
	siteURL := *p.API.GetConfig().ServiceSettings.SiteURL
	return fmt.Sprintf("%s/plugins/%s/api/v1/oauth/connect", siteURL, manifest.Id)
}

func (p *Plugin) buildAuthURL(state string) string {
	config := p.getConfiguration()
	siteURL := *p.API.GetConfig().ServiceSettings.SiteURL
	redirectURI := fmt.Sprintf("%s/plugins/%s/api/v1/oauth/callback", siteURL, manifest.Id)

	params := url.Values{
		"client_id":     {config.GoogleClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {calendarScope},
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
	siteURL := *p.API.GetConfig().ServiceSettings.SiteURL
	redirectURI := fmt.Sprintf("%s/plugins/%s/api/v1/oauth/callback", siteURL, manifest.Id)

	data := url.Values{
		"code":          {code},
		"client_id":     {config.GoogleClientID},
		"client_secret": {config.GoogleClientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}

	resp, err := httpClient.PostForm(googleTokenURL, data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to exchange code for token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read token response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, errors.Wrap(err, "failed to parse token response")
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
		return nil, errors.Wrap(err, "failed to refresh token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read refresh response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, errors.Wrap(err, "failed to parse refresh response")
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
	token, err := p.kvstore.GetOAuth2Token(userID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, nil
	}

	// Refresh if token expires within 1 minute
	if time.Until(token.Expiry) < time.Minute {
		token, err = p.refreshToken(token)
		if err != nil {
			return nil, errors.Wrap(err, "failed to refresh expired token")
		}
		if err := p.kvstore.StoreOAuth2Token(userID, token); err != nil {
			p.API.LogError("Failed to store refreshed token", "error", err.Error())
		}
	}

	return token, nil
}

type calendarEvent struct {
	Summary        string          `json:"summary"`
	Start          *eventDateTime  `json:"start"`
	End            *eventDateTime  `json:"end"`
	ConferenceData *conferenceData `json:"conferenceData"`
}

type eventDateTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type conferenceData struct {
	CreateRequest *createConferenceRequest `json:"createRequest"`
}

type createConferenceRequest struct {
	RequestID             string                 `json:"requestId"`
	ConferenceSolutionKey *conferenceSolutionKey `json:"conferenceSolutionKey"`
}

type conferenceSolutionKey struct {
	Type string `json:"type"`
}

type calendarEventResponse struct {
	HangoutLink    string                      `json:"hangoutLink"`
	ConferenceData *conferenceDataResponse      `json:"conferenceData"`
}

type conferenceDataResponse struct {
	EntryPoints []entryPoint `json:"entryPoints"`
}

type entryPoint struct {
	EntryPointType string `json:"entryPointType"`
	URI            string `json:"uri"`
}

func (p *Plugin) createMeeting(token *kvstore.OAuth2Token, topic string) (string, error) {
	if topic == "" {
		topic = "Google Meet Meeting"
	}

	now := time.Now()
	event := calendarEvent{
		Summary: topic,
		Start: &eventDateTime{
			DateTime: now.Format(time.RFC3339),
			TimeZone: "UTC",
		},
		End: &eventDateTime{
			DateTime: now.Add(1 * time.Hour).Format(time.RFC3339),
			TimeZone: "UTC",
		},
		ConferenceData: &conferenceData{
			CreateRequest: &createConferenceRequest{
				RequestID: uuid.New().String(),
				ConferenceSolutionKey: &conferenceSolutionKey{
					Type: "hangoutsMeet",
				},
			},
		},
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal event")
	}

	reqURL := googleCalendarURL + "/calendars/primary/events?conferenceDataVersion=1"
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(eventJSON))
	if err != nil {
		return "", errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to create calendar event")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("calendar API returned status %d: %s", resp.StatusCode, string(body))
	}

	var eventResp calendarEventResponse
	if err := json.Unmarshal(body, &eventResp); err != nil {
		return "", errors.Wrap(err, "failed to parse calendar response")
	}

	// Try hangoutLink first, then conference data entry points
	if eventResp.HangoutLink != "" {
		return eventResp.HangoutLink, nil
	}

	if eventResp.ConferenceData != nil {
		for _, ep := range eventResp.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" && strings.Contains(ep.URI, "meet.google.com") {
				return ep.URI, nil
			}
		}
	}

	return "", errors.New("no Google Meet link found in calendar event response")
}
