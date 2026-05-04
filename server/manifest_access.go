// Copyright (c) 2026-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import "sync"

var manifestMu sync.RWMutex

func manifestID() string {
	manifestMu.RLock()
	defer manifestMu.RUnlock()

	if manifest == nil {
		return ""
	}

	return manifest.Id
}

func setManifestSettingsHeader(header string) {
	manifestMu.Lock()
	defer manifestMu.Unlock()

	if manifest == nil || manifest.SettingsSchema == nil {
		return
	}

	manifest.SettingsSchema.Header = header
}
