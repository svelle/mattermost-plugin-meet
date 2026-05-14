// Copyright (c) 2026-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"errors"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-google-meet/server/command"
	"github.com/mattermost/mattermost-plugin-google-meet/server/store/kvstore"
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
	meetURL, spaceName, err := p.createMeeting(token, topic)
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

	createdPost, appErr := p.API.CreatePost(post)
	if appErr != nil {
		return fmt.Errorf("failed to create post: %w", appErr)
	}

	// Store an ad-hoc mapping so the polling loop can post recording / transcript /
	// smart-note artifacts as replies to this post without an explicit subscription.
	if spaceName != "" {
		kvStore := p.getKVStore()
		entry := &kvstore.AdHocMeetingPost{
			RootPostID: createdPost.Id,
			ChannelID:  channelID,
			UserID:     userID,
		}
		if storeErr := kvStore.StoreAdHocMeetingPost(spaceName, entry); storeErr != nil {
			p.API.LogWarn("StartMeeting: failed to store ad-hoc meeting post", "space", spaceName, "error", storeErr.Error())
		} else if storeErr := kvStore.AddToAdHocIndex(spaceName); storeErr != nil {
			p.API.LogWarn("StartMeeting: failed to add to ad-hoc index", "space", spaceName, "error", storeErr.Error())
		}
	}

	return nil
}
