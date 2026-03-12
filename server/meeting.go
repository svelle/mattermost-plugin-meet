package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

// ErrNeedsReconnect indicates the user must re-connect their Google account.
var ErrNeedsReconnect = errors.New("needs_reconnect")

// MeetingStarter is used by the command handler to start meetings.
type MeetingStarter interface {
	StartMeeting(userID, channelID, topic string) error
	GetConnectURL() string
	IsUserConnected(userID string) (bool, error)
	IsPluginConfigured() bool
	IsUserAdmin(userID string) (bool, error)
	GetPluginConfigureURL() string
}

func (p *Plugin) StartMeeting(userID, channelID, topic string) error {
	if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionCreatePost) {
		return errors.New("you don't have permission to create posts in this channel")
	}

	p.API.LogDebug("StartMeeting: getting valid token", "user_id", userID)
	token, err := p.getValidToken(userID)
	if err != nil {
		return errors.Wrap(err, "failed to get user token")
	}
	if token == nil {
		return errors.New("user not connected to Google")
	}

	p.API.LogDebug("StartMeeting: creating Google Meet meeting", "user_id", userID)
	meetURL, err := p.createMeeting(token, topic)
	if err != nil {
		if errors.Is(err, ErrInsufficientScopes) {
			// Token has old scopes — delete it so the user re-authenticates with the correct scope
			_ = p.kvstore.DeleteOAuth2Token(userID)
			return ErrNeedsReconnect
		}
		return errors.Wrap(err, "failed to create Google Meet meeting")
	}
	p.API.LogDebug("StartMeeting: meeting created", "user_id", userID, "meet_url", meetURL)

	message := "I have started a meeting"
	if topic != "" {
		message = fmt.Sprintf("I have started a meeting: **%s**", topic)
	}

	post := &model.Post{
		UserId:    userID,
		ChannelId: channelID,
		Message:   message,
		Type:      "custom_google_meet",
		Props: model.StringInterface{
			"meeting_link":  meetURL,
			"meeting_topic": topic,
		},
	}

	_, appErr := p.API.CreatePost(post)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to create post")
	}

	return nil
}

func (p *Plugin) GetConnectURL() string {
	return p.getOAuth2ConnectURL()
}

func (p *Plugin) IsUserConnected(userID string) (bool, error) {
	token, err := p.kvstore.GetOAuth2Token(userID)
	if err != nil {
		return false, err
	}
	return token != nil, nil
}

func (p *Plugin) IsPluginConfigured() bool {
	return p.getConfiguration().IsValid() == nil
}

func (p *Plugin) IsUserAdmin(userID string) (bool, error) {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return false, errors.Wrap(appErr, "failed to get user")
	}
	return user.IsSystemAdmin(), nil
}

func (p *Plugin) GetPluginConfigureURL() string {
	siteURL := *p.API.GetConfig().ServiceSettings.SiteURL
	return fmt.Sprintf("%s/admin_console/plugins/plugin_%s", siteURL, manifest.Id)
}
