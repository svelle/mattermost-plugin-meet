package main

import (
	"errors"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-google-meet/server/command"
)

// ErrNoChannelPermission indicates the user cannot create posts in the channel.
var ErrNoChannelPermission = errors.New("no permission to create posts in this channel")

func (p *Plugin) StartMeeting(userID, channelID, topic string) error {
	if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionCreatePost) {
		return ErrNoChannelPermission
	}

	if p.getConfiguration().RestrictMeetingCreation {
		channel, appErr := p.API.GetChannel(channelID)
		if appErr != nil {
			return fmt.Errorf("failed to get channel: %w", appErr)
		}
		if channel.Type == model.ChannelTypeOpen {
			return command.ErrPublicChannelRestricted
		}
	}

	p.API.LogDebug("StartMeeting: getting valid token", "user_id", userID)
	token, err := p.getValidToken(userID)
	if err != nil {
		return fmt.Errorf("failed to get user token: %w", err)
	}
	if token == nil {
		return errors.New("user not connected to Google")
	}

	p.API.LogDebug("StartMeeting: creating Google Meet meeting", "user_id", userID)
	meetURL, err := p.createMeeting(token, topic)
	if err != nil {
		if errors.Is(err, ErrInsufficientScopes) {
			// Token has old scopes — delete it so the user re-authenticates with the correct scope
			store, storeErr := p.getOAuthKVStore()
			if storeErr != nil {
				p.API.LogWarn("OAuth storage unavailable while deleting token after insufficient scopes", "user_id", userID, "error", storeErr.Error())
			} else if delErr := store.DeleteOAuth2Token(userID); delErr != nil {
				p.API.LogWarn("Failed to delete OAuth token after insufficient scopes", "user_id", userID, "error", delErr.Error())
			}
			return command.ErrNeedsReconnect
		}
		return fmt.Errorf("failed to create Google Meet meeting: %w", err)
	}
	p.API.LogDebug("StartMeeting: meeting created", "user_id", userID)

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
		return fmt.Errorf("failed to create post: %w", appErr)
	}

	return nil
}
