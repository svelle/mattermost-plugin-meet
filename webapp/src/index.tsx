import manifest from 'manifest';
import React from 'react';
import type {Store} from 'redux';

import type {GlobalState} from '@mattermost/types/store';

import {makeStyleFromTheme} from 'mattermost-redux/utils/theme_utils';

import type {PluginRegistry} from 'types/mattermost-webapp';

/* eslint-disable react/prop-types */

type ExternalLinkProps = React.AnchorHTMLAttributes<HTMLAnchorElement> & {
    children: React.ReactNode;
    href: string;
};

const ExternalLink = ({children, href, rel, target, ...props}: ExternalLinkProps) => (
    <a
        {...props}
        href={href}
        rel={rel || 'noopener noreferrer'}
        target={target || '_blank'}
    >
        {children}
    </a>
);

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

const VideoCameraIcon = () => (
    <svg
        width='16'
        height='16'
        viewBox='0 0 24 24'
        fill='currentColor'
        xmlns='http://www.w3.org/2000/svg'
    >
        <path
            d='M17 10.5V7c0-.55-.45-1-1-1H4c-.55 0-1 .45-1 1v10c0 .55.45 1 1 1h12c.55 0 1-.45 1-1v-3.5l4 4v-11l-4 4z'
        />
    </svg>
);

type ConnectPromptState = {
    connectURL: string;
    isOpen: boolean;
    message: string;
};

const defaultConnectPromptState: ConnectPromptState = {
    connectURL: '',
    isOpen: false,
    message: '',
};

let connectPromptState = defaultConnectPromptState;
const connectPromptListeners = new Set<(state: ConnectPromptState) => void>();

const setConnectPromptState = (nextState: ConnectPromptState) => {
    connectPromptState = nextState;
    connectPromptListeners.forEach((listener) => listener(nextState));
};

const hideConnectPrompt = () => {
    setConnectPromptState(defaultConnectPromptState);
};

const showConnectPrompt = (message: string, connectURL: string) => {
    setConnectPromptState({
        connectURL,
        isOpen: true,
        message,
    });
};

const ConnectPromptModal = () => {
    const [prompt, setPrompt] = React.useState(connectPromptState);

    React.useEffect(() => {
        connectPromptListeners.add(setPrompt);
        return () => {
            connectPromptListeners.delete(setPrompt);
        };
    }, []);

    if (!prompt.isOpen) {
        return null;
    }

    return (
        <div
            aria-modal='true'
            role='dialog'
            style={{
                alignItems: 'center',
                backgroundColor: 'rgba(0, 0, 0, 0.5)',
                bottom: 0,
                display: 'flex',
                justifyContent: 'center',
                left: 0,
                position: 'fixed',
                right: 0,
                top: 0,
                zIndex: 9999,
            }}
        >
            <div
                style={{
                    backgroundColor: '#fff',
                    borderRadius: '8px',
                    boxShadow: '0 12px 32px rgba(0, 0, 0, 0.24)',
                    maxWidth: '420px',
                    padding: '24px',
                    width: '100%',
                }}
            >
                <h3 style={{margin: '0 0 12px'}}>
                    {'Connect Google Meet'}
                </h3>
                <p style={{margin: '0 0 20px'}}>
                    {prompt.message}
                </p>
                <div
                    style={{
                        display: 'flex',
                        gap: '12px',
                        justifyContent: 'flex-end',
                    }}
                >
                    <button
                        onClick={hideConnectPrompt}
                        style={{
                            backgroundColor: '#f2f3f5',
                            border: '1px solid #d3d3d3',
                            borderRadius: '4px',
                            cursor: 'pointer',
                            padding: '8px 16px',
                        }}
                        type='button'
                    >
                        {'Cancel'}
                    </button>
                    <button
                        onClick={() => {
                            if (prompt.connectURL) {
                                window.open(prompt.connectURL, '_blank', 'noopener,noreferrer');
                            }
                            hideConnectPrompt();
                        }}
                        style={{
                            backgroundColor: '#1c58d9',
                            border: 'none',
                            borderRadius: '4px',
                            color: '#fff',
                            cursor: 'pointer',
                            padding: '8px 16px',
                        }}
                        type='button'
                    >
                        {'Connect'}
                    </button>
                </div>
            </div>
        </div>
    );
};

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
            alignItems: 'center',
            color: theme.buttonColor,
            display: 'inline-flex',
            paddingRight: '8px',
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
    const props = post.props || {};
    const meetingLink = props.meeting_link || '';
    const meetingTopic = props.meeting_topic || '';

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
            <ExternalLink
                href={meetingLink}
            >
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
            <span style={style.buttonIcon}>
                <VideoCameraIcon/>
            </span>
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

export default class Plugin {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    public async initialize(registry: PluginRegistry, store: Store<GlobalState>) {
        registry.registerRootComponent(ConnectPromptModal);

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

        const startMeeting = async (channel: {id: string}) => {
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
                showConnectPrompt(
                    'You need to connect your Google account to start a meeting. Connect now?',
                    data.connect_url,
                );
                return;
            }
            if (data.error === 'meeting_failed') {
                // The plugin registry exposes no ephemeral-message helper, so this
                // client-only workaround dispatches a `system_ephemeral` post
                // straight to Redux. It is not persisted to the server, other
                // clients will not see it, and refreshing the page can clear it.
                const currentUserId = store.getState().entities.users.currentUserId;
                const timestamp = Date.now();
                const randomSuffix = Math.random().toString(36).substring(2, 8);
                store.dispatch({
                    type: 'RECEIVED_NEW_POST',
                    data: {
                        id: `meet_error_${timestamp}_${randomSuffix}`,
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
        };

        registry.registerChannelHeaderButtonAction(
            <GoogleMeetIcon/>,
            startMeeting as unknown as () => void,
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
