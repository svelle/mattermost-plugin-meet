import manifest from 'manifest';
import {getCsrfToken} from 'utils/csrf';

export interface ConfigStatusResponse {
    configured: boolean;
    is_admin: boolean;
}

export interface CreateMeetingResponse {
    status?: string;
    reason?: string;
    message?: string;
    error?: string;
}

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

export const getConfigStatus = async (): Promise<ConfigStatusResponse> => {
    const statusResp = await doFetch(`/plugins/${manifest.id}/api/v1/config/status`);
    if (!statusResp.ok) {
        throw new Error(`Failed to load config status (HTTP ${statusResp.status}).`);
    }
    return statusResp.json();
};

export const createMeeting = async (channelID: string): Promise<CreateMeetingResponse> => {
    let resp: Response;
    try {
        resp = await doFetch(`/plugins/${manifest.id}/api/v1/meeting`, {
            method: 'POST',
            body: JSON.stringify({channel_id: channelID}),
        });
    } catch {
        throw new Error('Unable to reach the server to start a Google Meet meeting. Please try again.');
    }

    const contentType = resp.headers.get('Content-Type') || '';
    if (!contentType.includes('application/json')) {
        const message = resp.ok ?
            'Received an unexpected response from the server while starting a Google Meet meeting.' :
            `Failed to start a Google Meet meeting (HTTP ${resp.status}). Please try again.`;
        throw new Error(message);
    }

    let data: CreateMeetingResponse;
    try {
        data = await resp.json();
    } catch {
        throw new Error('Received an unreadable response from the server while starting a Google Meet meeting.');
    }

    if (!resp.ok) {
        const serverContext = data.reason || data.message || data.error;
        const message = serverContext ?
            `Failed to start a Google Meet meeting (HTTP ${resp.status}): ${serverContext}. Please try again.` :
            `Failed to start a Google Meet meeting (HTTP ${resp.status}). Please try again.`;
        throw new Error(message);
    }

    return data;
};
