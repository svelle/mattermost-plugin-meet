import React from 'react';

import {makeStyleFromTheme} from 'mattermost-redux/utils/theme_utils';

import ExternalLink from 'components/external_link';

import {VideoCameraIcon} from './icons';

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
    const meetingActions = meetingLink ? (
        <>
            <span>
                {'Meeting URL: '}
                <ExternalLink href={meetingLink}>
                    {meetingLink}
                </ExternalLink>
            </span>
            <div>
                <div style={style.body}>
                    <ExternalLink
                        className='btn btn-primary'
                        style={style.button}
                        href={meetingLink}
                    >
                        <VideoCameraIcon style={style.buttonIcon}/>
                        {'JOIN MEETING'}
                    </ExternalLink>
                </div>
            </div>
        </>
    ) : null;

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
                    {meetingActions}
                </div>
            </div>
        </div>
    );
};

export default PostTypeGoogleMeet;
