// Copyright (c) 2026-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"errors"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-google-meet/server/command"
)

const websocketEventMeetingStarted = "meeting_started"

// ErrNoChannelPermission indicates the user cannot create posts in the channel.
var ErrNoChannelPermission = errors.New("no permission to create posts in this channel")

// StartMeeting creates a Google Meet space, posts to the channel, notifies the
// starter's clients via WebSocket (for opening the join URL), and returns the meet URL.
func (p *Plugin) StartMeeting(userID, channelID, topic string) (string, error) {
	if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionCreatePost) {
		return "", ErrNoChannelPermission
	}

	if p.getConfiguration().RestrictMeetingCreation {
		channel, appErr := p.API.GetChannel(channelID)
		if appErr != nil {
			return fmt.Errorf("failed to get channel: %w", appErr)
		}
		if channel.Type == model.ChannelTypeOpen {
			return "", command.ErrPublicChannelRestricted
		}
	}

	p.API.LogDebug("StartMeeting: getting valid token", "user_id", userID)
	token, err := p.getValidToken(userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user token: %w", err)
	}
	if token == nil {
		return "", errors.New("user not connected to Google")
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
			return "", command.ErrNeedsReconnect
		}
		return "", fmt.Errorf("failed to create Google Meet meeting: %w", err)
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
		return "", fmt.Errorf("failed to create post: %w", appErr)
	}

	p.API.PublishWebSocketEvent(websocketEventMeetingStarted, map[string]any{
		"meeting_url": meetURL,
	}, &model.WebsocketBroadcast{UserId: userID})

	return meetURL, nil
}
