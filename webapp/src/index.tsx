import manifest from 'manifest';
import type {Store} from 'redux';

import type {GlobalState} from '@mattermost/types/store';

import type {PluginRegistry} from 'types/mattermost-webapp';

import React from 'react';

const GoogleMeetIcon = () => (
    <svg
        viewBox='0 0 24 24'
        width='16px'
        height='16px'
        fill='currentColor'
    >
        <path d='M17 10.5V7c0-.55-.45-1-1-1H7c-.55 0-1 .45-1 1v10c0 .55.45 1 1 1h9c.55 0 1-.45 1-1v-3.5l4 4v-11l-4 4z'/>
    </svg>
);

export default class Plugin {
    public async initialize(registry: PluginRegistry, store: Store<GlobalState>) {
        registry.registerChannelHeaderButtonAction(
            <GoogleMeetIcon/>,
            async (channel: {id: string}) => {
                const resp = await fetch(`/plugins/${manifest.id}/api/v1/meeting`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({channel_id: channel.id}),
                });

                if (!resp.ok) {
                    console.error('Failed to create meeting:', resp.statusText); // eslint-disable-line no-console
                    return;
                }

                const data = await resp.json();
                if (data.error === 'not_connected') {
                    window.open(data.connect_url, '_blank');
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
