// Copyright (c) 2026-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"time"

	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"

	"github.com/mattermost/mattermost-plugin-google-meet/server/store/kvstore"
)

// startPoller launches the background polling goroutine.
// It is safe to call from OnActivate; the goroutine is stopped via stopPoller.
func (p *Plugin) startPoller() {
	if !p.getConfiguration().EnableConferenceArtifactPosts {
		p.API.LogInfo("Google Meet poller not started: EnableConferenceArtifactPosts is disabled")
		return
	}

	intervalSec := p.getConfiguration().pollInterval()
	p.API.LogInfo("Starting Google Meet poller", "interval_seconds", intervalSec)

	p.pollerStop = make(chan struct{})
	go func() {
		interval := time.Duration(intervalSec) * time.Second
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-p.pollerStop:
				return
			case <-ticker.C:
				p.runPollCycle()
			}
		}
	}()
}

// stopPoller signals the polling goroutine to exit.
func (p *Plugin) stopPoller() {
	if p.pollerStop != nil {
		close(p.pollerStop)
		p.pollerStop = nil
	}
}

// runPollCycle is the work done on each tick. It acquires a distributed mutex
// so that only one node in an HA cluster processes subscriptions at a time.
func (p *Plugin) runPollCycle() {
	mutex, err := cluster.NewMutex(p.API, "com.mattermost.google-meet.poll")
	if err != nil {
		p.API.LogError("Failed to create polling mutex", "error", err.Error())
		return
	}
	mutex.Lock()
	defer mutex.Unlock()

	store := p.getKVStore()
	if store == nil {
		p.API.LogWarn("Skipping poll cycle: KV store not initialized (plugin not fully configured)")
		return
	}

	spaceIDs, err := store.ListAllSubscriptionSpaceIDs()
	if err != nil {
		p.API.LogError("Failed to list subscription space IDs during poll", "error", err.Error())
		return
	}

	for _, spaceID := range spaceIDs {
		sub, err := store.GetSubscription(spaceID)
		if err != nil {
			p.API.LogWarn("Failed to load subscription during poll", "space_id", spaceID, "error", err.Error())
			continue
		}
		if sub == nil {
			p.API.LogWarn("Subscription index entry has no stored record", "space_id", spaceID)
			continue
		}
		p.pollSubscription(store, sub)
	}

	p.pollAdHocMeetings(store)
}

// pollSubscription handles one subscription: finds new conferences and checks active ones for artifacts.
func (p *Plugin) pollSubscription(store kvstore.KVStore, sub *kvstore.Subscription) {
	token, err := p.getValidToken(sub.CreatedBy)
	if err != nil {
		p.API.LogWarn("Skipping subscription poll: token lookup failed", "space_id", sub.SpaceID, "created_by", sub.CreatedBy, "error", err.Error())
		return
	}
	if token == nil {
		p.API.LogDebug("Skipping subscription poll: user is not connected to Google", "space_id", sub.SpaceID, "created_by", sub.CreatedBy)
		return
	}

	records, err := p.listConferenceRecords(token, sub.SpaceID, sub.LastSeenConferenceStart)
	if err != nil {
		p.API.LogWarn("Failed to list conference records", "space_id", sub.SpaceID, "error", err.Error())
		records = nil
	}

	for i := range records {
		record := &records[i]
		state, err := store.GetConferencePostState(record.Name)
		if err != nil {
			p.API.LogWarn("Failed to get conference post state", "conference", record.Name, "error", err.Error())
			continue
		}
		if state != nil {
			continue
		}

		postID, err := p.postConferenceStarted(sub, record)
		if err != nil {
			p.API.LogWarn("Failed to post conference started", "conference", record.Name, "error", err.Error())
			continue
		}
		p.API.LogInfo("Posted new Google Meet conference notification", "conference", record.Name, "space_id", sub.SpaceID, "channel_id", sub.ChannelID, "root_post_id", postID)
		state = &kvstore.ConferencePostState{
			RootPostID: postID,
			ChannelID:  sub.ChannelID,
		}
		if err := store.StoreConferencePostState(record.Name, state); err != nil {
			p.API.LogWarn("Failed to store conference post state", "conference", record.Name, "error", err.Error())
		}

		// Advance the high-water mark so future polls skip this record.
		if record.StartTime != nil && record.StartTime.After(sub.LastSeenConferenceStart) {
			sub.LastSeenConferenceStart = *record.StartTime
		}
		sub.ActiveConferenceIDs = appendIfMissing(sub.ActiveConferenceIDs, record.Name)
		if err := store.StoreSubscription(sub); err != nil {
			p.API.LogWarn("Failed to update subscription state", "space_id", sub.SpaceID, "error", err.Error())
		}
	}

	if !p.getConfiguration().EnableConferenceArtifactPosts {
		return
	}

	for _, confName := range sub.ActiveConferenceIDs {
		p.pollConferenceArtifacts(store, token, sub, confName)
	}
}

// pollConferenceArtifacts checks a single conference record for new recordings/transcripts/smart notes.
// Monitoring stops implicitly when the conference's KV state entry expires (TTL).
func (p *Plugin) pollConferenceArtifacts(store kvstore.KVStore, token *kvstore.OAuth2Token, sub *kvstore.Subscription, confName string) bool {
	state, err := store.GetConferencePostState(confName)
	if err != nil {
		p.API.LogWarn("Failed to get conference post state during artifact poll", "conference", confName, "error", err.Error())
		return true
	}
	if state == nil {
		return true
	}

	changed := false

	recordings, err := p.listRecordings(token, confName)
	if err != nil {
		p.API.LogWarn("Failed to list recordings", "conference", confName, "error", err.Error())
	}
	for i := range recordings {
		rec := &recordings[i]
		if rec.State != meetStateFileGenerated {
			continue
		}
		if containsString(state.PostedRecordingIDs, rec.Name) {
			continue
		}
		if err := p.postRecording(state.ChannelID, state.RootPostID, rec); err != nil {
			p.API.LogWarn("Failed to post recording", "recording", rec.Name, "error", err.Error())
			continue
		}
		p.API.LogInfo("Posted recording to thread", "recording", rec.Name, "conference", confName, "root_post_id", state.RootPostID)
		state.PostedRecordingIDs = append(state.PostedRecordingIDs, rec.Name)
		changed = true
	}

	transcripts, err := p.listTranscripts(token, confName)
	if err != nil {
		p.API.LogWarn("Failed to list transcripts", "conference", confName, "error", err.Error())
	}
	for i := range transcripts {
		tr := &transcripts[i]
		if tr.State != meetStateFileGenerated {
			continue
		}
		if containsString(state.PostedTranscriptIDs, tr.Name) {
			continue
		}
		if err := p.postTranscript(token, state.ChannelID, state.RootPostID, tr); err != nil {
			p.API.LogWarn("Failed to post transcript", "transcript", tr.Name, "error", err.Error())
			continue
		}
		p.API.LogInfo("Posted transcript to thread", "transcript", tr.Name, "conference", confName, "root_post_id", state.RootPostID)
		state.PostedTranscriptIDs = append(state.PostedTranscriptIDs, tr.Name)
		changed = true
	}

	smartNotes, err := p.listSmartNotes(token, confName)
	if err != nil {
		p.API.LogWarn("Failed to list smart notes", "conference", confName, "error", err.Error())
	}
	for i := range smartNotes {
		sn := &smartNotes[i]
		if sn.State != meetStateFileGenerated {
			continue
		}
		if containsString(state.PostedSmartNoteIDs, sn.Name) {
			continue
		}
		if err := p.postSmartNote(state.ChannelID, state.RootPostID, sn); err != nil {
			p.API.LogWarn("Failed to post smart note", "smart_note", sn.Name, "error", err.Error())
			continue
		}
		p.API.LogInfo("Posted smart note to thread", "smart_note", sn.Name, "conference", confName, "root_post_id", state.RootPostID)
		state.PostedSmartNoteIDs = append(state.PostedSmartNoteIDs, sn.Name)
		changed = true
	}

	if changed {
		if err := store.StoreConferencePostState(confName, state); err != nil {
			p.API.LogWarn("Failed to update conference post state", "conference", confName, "error", err.Error())
		}
	}

	return false
}

// pollAdHocMeetings checks all ad-hoc meetings (started via /meet start) for new artifacts.
// Unlike subscriptions, ad-hoc entries are pinned to a specific post that already exists as
// the root, so there is no need to create a conference-started post — we reuse the one
// created by StartMeeting.
func (p *Plugin) pollAdHocMeetings(store kvstore.KVStore) {
	spaceIDs, err := store.ListAdHocSpaceIDs()
	if err != nil {
		p.API.LogError("Failed to list ad-hoc space IDs during poll", "error", err.Error())
		return
	}

	for _, spaceID := range spaceIDs {
		entry, err := store.GetAdHocMeetingPost(spaceID)
		if err != nil {
			p.API.LogWarn("Failed to get ad-hoc meeting post", "space_id", spaceID, "error", err.Error())
			continue
		}
		if entry == nil {
			if err := store.RemoveFromAdHocIndex(spaceID); err != nil {
				p.API.LogWarn("Failed to remove expired ad-hoc entry from index", "space_id", spaceID, "error", err.Error())
			}
			continue
		}

		token, err := p.getValidToken(entry.UserID)
		if err != nil {
			p.API.LogWarn("Skipping ad-hoc poll: token lookup failed", "space_id", spaceID, "user_id", entry.UserID, "error", err.Error())
			continue
		}
		if token == nil {
			p.API.LogDebug("Skipping ad-hoc poll: user is not connected to Google", "space_id", spaceID, "user_id", entry.UserID)
			continue
		}

		records, err := p.listConferenceRecords(token, spaceID, time.Time{})
		if err != nil {
			p.API.LogWarn("Failed to list conference records for ad-hoc space", "space_id", spaceID, "error", err.Error())
			continue
		}

		for i := range records {
			record := &records[i]
			state, err := store.GetConferencePostState(record.Name)
			if err != nil {
				p.API.LogWarn("Failed to get conference post state for ad-hoc meeting", "conference", record.Name, "error", err.Error())
				continue
			}

			if state == nil {
				// Pin the conference to the existing /meet start post instead of creating a new one.
				state = &kvstore.ConferencePostState{
					RootPostID: entry.RootPostID,
					ChannelID:  entry.ChannelID,
				}
				if err := store.StoreConferencePostState(record.Name, state); err != nil {
					p.API.LogWarn("Failed to store conference post state for ad-hoc meeting", "conference", record.Name, "error", err.Error())
					continue
				}
			}

			syntheticSub := &kvstore.Subscription{
				SpaceID:   spaceID,
				ChannelID: entry.ChannelID,
			}
			p.pollConferenceArtifacts(store, token, syntheticSub, record.Name)
		}
	}
}

func appendIfMissing(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
