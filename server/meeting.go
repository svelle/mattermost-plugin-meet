package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

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
	token, err := p.getValidToken(userID)
	if err != nil {
		return errors.Wrap(err, "failed to get user token")
	}
	if token == nil {
		return errors.New("user not connected to Google")
	}

	meetURL, err := p.createMeeting(token, topic)
	if err != nil {
		return errors.Wrap(err, "failed to create Google Meet meeting")
	}

	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to get user")
	}

	displayName := user.GetDisplayName(model.ShowNicknameFullName)

	message := fmt.Sprintf("@%s started a new meeting", user.Username)
	if topic != "" {
		message = fmt.Sprintf("@%s started a new meeting: **%s**", user.Username, topic)
	}

	post := &model.Post{
		UserId:    userID,
		ChannelId: channelID,
		Message:   message,
		Type:      "custom_google_meet",
		Props: model.StringInterface{
			"meeting_link":    meetURL,
			"meeting_topic":   topic,
			"from_webhook":    "true",
			"override_username": displayName,
			"attachments": []*model.SlackAttachment{
				{
					Fallback: fmt.Sprintf("Join Google Meet: %s", meetURL),
					Title:    "Join Google Meet",
					TitleLink: meetURL,
					Color:    "#00897B",
					Text:     fmt.Sprintf(":video_camera: [Join Google Meet](%s)", meetURL),
				},
			},
		},
	}

	_, appErr = p.API.CreatePost(post)
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
