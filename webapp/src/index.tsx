import manifest from 'manifest';
import React from 'react';
import type {Store} from 'redux';

import type {GlobalState} from '@mattermost/types/store';

import {makeStyleFromTheme} from 'mattermost-redux/utils/theme_utils';

import ExternalLink from 'components/external_link';

import type {PluginRegistry} from 'types/mattermost-webapp';

const GoogleMeetIcon = () => (
    <svg
        xmlns='http://www.w3.org/2000/svg'
        fill='none'
        viewBox='0 0 87.5 72'
        width='16px'
        height='13px'
    >
        <path
            fill='#00832d'
            d='M49.5 36l8.53 9.75 11.47 7.33 2-17.02-2-16.64-11.69 6.44z'
        />
        <path
            fill='#0066da'
            d='M0 51.5V66c0 3.315 2.685 6 6 6h14.5l3-10.96-3-9.54-9.95-3z'
        />
        <path
            fill='#e94235'
            d='M20.5 0L0 20.5l10.55 3 9.95-3 2.95-9.41z'
        />
        <path
            fill='#2684fc'
            d='M20.5 20.5H0v31h20.5z'
        />
        <path
            fill='#00ac47'
            d='M82.6 8.68L69.5 19.42v33.66l13.16 10.79c1.97 1.54 4.85.135 4.85-2.37V11c0-2.535-2.945-3.925-4.91-2.32zM49.5 36v15.5h-29V72h43c3.315 0 6-2.685 6-6V53.08z'
        />
        <path
            fill='#ffba00'
            d='M63.5 0h-43v20.5h29V36l20-16.57V6c0-3.315-2.685-6-6-6z'
        />
    </svg>
);

const VideoCameraIcon = ({style}: {style?: React.CSSProperties}) => (
    <svg
        xmlns='http://www.w3.org/2000/svg'
        viewBox='0 0 24 24'
        width='16px'
        height='16px'
        fill='currentColor'
        style={style}
    >
        <path d='M17 10.5V7c0-.55-.45-1-1-1H4c-.55 0-1 .45-1 1v10c0 .55.45 1 1 1h12c.55 0 1-.45 1-1v-3.5l4 4v-11l-4 4z'/>
    </svg>
);

const getStyle = makeStyleFromTheme((theme: Record<string, string>) => {
    return {
        body: {
            overflowX: 'auto',
            overflowY: 'hidden',
            paddingRight: '5px',
            width: '100%',
        },
        title: {
            fontWeight: '600',
        },
        button: {
            fontFamily: 'Open Sans',
            fontSize: '12px',
            fontWeight: 'bold',
            letterSpacing: '1px',
            lineHeight: '19px',
            marginTop: '12px',
            borderRadius: '4px',
            color: theme.buttonColor,
        },
        buttonIcon: {
            paddingRight: '8px',
            verticalAlign: 'text-bottom',
        },
    };
});

interface PostTypeGoogleMeetProps {
    post: {
        message: string;
        props: {
            meeting_link?: string;
            meeting_topic?: string;
        };
    };
    theme: Record<string, string>;
}

const PostTypeGoogleMeet = ({post, theme}: PostTypeGoogleMeetProps) => {
    const style = getStyle(theme);
    const {meeting_link: meetingLink = '', meeting_topic: meetingTopic = ''} = post.props || {};

    const {formatText, messageHtmlToComponent} = (window as any).PostUtils || {};

    const renderMarkdown = (text: string) => {
        if (formatText && messageHtmlToComponent) {
            return messageHtmlToComponent(formatText(text, {atMentions: true}), false);
        }
        return text;
    };

    const preText = renderMarkdown(post.message);
    const title = meetingTopic ? renderMarkdown(meetingTopic) : 'Google Meet';

    const subtitle = (
        <span>
            {'Meeting URL: '}
            <ExternalLink href={meetingLink}>
                {meetingLink}
            </ExternalLink>
        </span>
    );

    const content = (
        <ExternalLink
            className='btn btn-primary'
            style={style.button}
            href={meetingLink}
        >
            <VideoCameraIcon style={style.buttonIcon}/>
            {'JOIN MEETING'}
        </ExternalLink>
    );

    return (
        <div className='attachment attachment--pretext'>
            <div className='attachment__thumb-pretext'>
                {preText}
            </div>
            <div className='attachment__content'>
                <div
                    className='clearfix attachment__container'
                    style={{borderLeftColor: '#00832d'}}
                >
                    <h5
                        className='mt-1'
                        style={style.title}
                    >
                        {title}
                    </h5>
                    {subtitle}
                    <div>
                        <div style={style.body}>
                            {content}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

const getCsrfToken = (): string => {
    const match = document.cookie.match(/MMCSRF=([^;]+)/);
    return match ? match[1] : '';
};

const doFetch = async (url: string, options: RequestInit = {}): Promise<Response> => {
    const {headers: optHeaders, ...restOptions} = options;
    return fetch(url, {
        credentials: 'include',
        ...restOptions,
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': getCsrfToken(),
            ...(optHeaders || {}),
        },
    });
};

const postEphemeralMessage = (store: Store<GlobalState>, channelID: string, message: string) => {
    const currentUserId = store.getState().entities.users.currentUserId;
    const timestamp = Date.now();
    const randomSuffix = Math.random().toString(36).substring(2, 8);

    // Plugins do not get a dedicated client-side ephemeral message API from the
    // registry, so we dispatch a local system_ephemeral post directly into Redux.
    // This is intentionally client-only: it is not persisted to the server, will
    // only appear for the current client session, and may disappear on refresh.
    store.dispatch({
        type: 'RECEIVED_NEW_POST',
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
        },
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

        // Register custom post type for Google Meet meeting posts
        registry.registerPostTypeComponent('custom_google_meet', PostTypeGoogleMeet);

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
                        window.open(data.configure_url, '_blank', 'noopener,noreferrer');
                    }
                    return;
                }
                if (data.error === 'not_connected') {
                    postEphemeralMessage(
                        store,
                        channel.id,
                        `You need to connect your Google account first. [Click here to connect](${data.connect_url}).`,
                    );
                    return;
                }
                if (data.error === 'meeting_failed') {
                    postEphemeralMessage(
                        store,
                        channel.id,
                        `Failed to create Google Meet meeting: ${data.message || 'Unknown error'}. Please try again.`,
                    );
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
