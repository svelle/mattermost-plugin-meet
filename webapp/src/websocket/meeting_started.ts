// Copyright (c) 2026-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {WebSocketMessage} from '@mattermost/client';

type MeetingStartedPayload = {
    meeting_url?: string;
};

function isAllowedGoogleMeetURL(raw: string): boolean {
    let u: URL;
    try {
        u = new URL(raw);
    } catch {
        return false;
    }
    if (u.protocol !== 'https:') {
        return false;
    }
    return u.hostname === 'meet.google.com';
}

export function handleMeetingStarted(msg: WebSocketMessage<MeetingStartedPayload>): void {
    const url = msg.data?.meeting_url;
    if (!url || typeof url !== 'string' || !isAllowedGoogleMeetURL(url)) {
        return;
    }
    window.open(url, '_blank', 'noopener,noreferrer');
}
