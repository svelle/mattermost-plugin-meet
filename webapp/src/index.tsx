import manifest from 'manifest';
import React from 'react';
import type {Store} from 'redux';

import type {GlobalState} from '@mattermost/types/store';

import type {PluginRegistry} from 'types/mattermost-webapp';

const GoogleMeetIcon = () => (
    <svg
        xmlns='http://www.w3.org/2000/svg'
        fill='none'
        viewBox='0 0 87.5 72'
        width='16px'
        height='13px'
    >
        <path fill='#00832d' d='M49.5 36l8.53 9.75 11.47 7.33 2-17.02-2-16.64-11.69 6.44z'/>
        <path fill='#0066da' d='M0 51.5V66c0 3.315 2.685 6 6 6h14.5l3-10.96-3-9.54-9.95-3z'/>
        <path fill='#e94235' d='M20.5 0L0 20.5l10.55 3 9.95-3 2.95-9.41z'/>
        <path fill='#2684fc' d='M20.5 20.5H0v31h20.5z'/>
        <path fill='#00ac47' d='M82.6 8.68L69.5 19.42v33.66l13.16 10.79c1.97 1.54 4.85.135 4.85-2.37V11c0-2.535-2.945-3.925-4.91-2.32zM49.5 36v15.5h-29V72h43c3.315 0 6-2.685 6-6V53.08z'/>
        <path fill='#ffba00' d='M63.5 0h-43v20.5h29V36l20-16.57V6c0-3.315-2.685-6-6-6z'/>
    </svg>
);

const getCsrfToken = (): string => {
    const match = document.cookie.match(/MMCSRF=([^;]+)/);
    return match ? match[1] : '';
};

const doFetch = async (url: string, options: RequestInit = {}): Promise<Response> => {
    return fetch(url, {
        credentials: 'include',
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': getCsrfToken(),
            ...options.headers,
        },
        ...options,
    });
};

export default class Plugin {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    public async initialize(registry: PluginRegistry, store: Store<GlobalState>) {
        // Check if the plugin is configured and whether the user is an admin
        let configured = false;
        let isAdmin = false;
        try {
            const statusResp = await doFetch(`/plugins/${manifest.id}/api/v1/config/status`);
            if (statusResp.ok) {
                const status = await statusResp.json();
                configured = status.configured;
                isAdmin = status.is_admin;
            }
        } catch {
            // If we can't check, fall through and register the button anyway
            configured = true;
        }

        // Hide the button entirely for non-admin users when not configured
        if (!configured && !isAdmin) {
            return;
        }

        registry.registerChannelHeaderButtonAction(
            <GoogleMeetIcon/>,
            async (channel: {id: string}) => {
                const resp = await doFetch(`/plugins/${manifest.id}/api/v1/meeting`, {
                    method: 'POST',
                    body: JSON.stringify({channel_id: channel.id}),
                });

                const data = await resp.json();
                if (data.error === 'not_configured') {
                    if (data.configure_url) {
                        window.open(data.configure_url, '_blank');
                    }
                    return;
                }
                if (data.error === 'not_connected') {
                    window.open(data.connect_url, '_blank');
                    return;
                }
                if (data.error === 'meeting_failed') {
                    store.dispatch({
                        type: 'RECEIVED_WEBAPP_PLUGIN',
                    });

                    // Post ephemeral-style error via the store
                    const currentUserId = store.getState().entities.users.currentUserId;
                    const timestamp = Date.now();
                    store.dispatch({
                        type: 'RECEIVED_NEW_POST',
                        data: {
                            id: `meet_error_${timestamp}`,
                            create_at: timestamp,
                            update_at: timestamp,
                            delete_at: 0,
                            user_id: currentUserId,
                            channel_id: channel.id,
                            message: `Failed to create Google Meet meeting: ${data.message || 'Unknown error'}. Please try again.`,
                            type: 'system_ephemeral',
                            props: {},
                        },
                    });
                }
            },
            'Start Google Meet',
            'Start a Google Meet meeting',
        );
    }
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void;
    }
}

window.registerPlugin(manifest.id, new Plugin());
