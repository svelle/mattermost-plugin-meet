import type {Store} from 'redux';

import type {GlobalState} from '@mattermost/types/store';

// The plugin registry does not expose the platform action constant, so keep a
// local fallback here instead of scattering the string literal across helpers.
const RECEIVED_NEW_POST = 'RECEIVED_NEW_POST';

export const postEphemeralMessage = (store: Store<GlobalState>, channelID: string, message: string) => {
    const currentUserId = store.getState().entities.users.currentUserId;
    const timestamp = Date.now();
    const randomSuffix = Math.random().toString(36).substring(2, 8);

    // Plugins still do not get a dedicated client-side ephemeral message API from
    // the registry, so transport-level fallbacks dispatch a local system_ephemeral
    // post into Redux. This is intentionally client-only, is not persisted to the
    // server, only appears in the current session, and may disappear on refresh.
    store.dispatch({
        type: RECEIVED_NEW_POST,
        data: {
            id: `meet_message_${timestamp}_${randomSuffix}`,
            create_at: timestamp,
            update_at: timestamp,
            delete_at: 0,
            user_id: currentUserId,
            channel_id: channelID,
            message,
            type: 'system_ephemeral',
            props: {},
            features: {crtEnabled: false},
        },
    });
};
