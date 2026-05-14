// Copyright (c) 2026-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import ExternalLink from 'components/external_link';

type Props = {
    post: {
        message: string;
        props: {
            meeting_code?: string;
            space_id?: string;
        };
    };
};

const PostTypeConference = ({post}: Props) => {
    const {meeting_code: meetingCode = '', space_id: spaceID = ''} = post.props || {};
    const meetURL = meetingCode ? `https://meet.google.com/${meetingCode}` : '';

    return (
        <div className='attachment attachment--pretext'>
            <div className='attachment__content'>
                <div
                    className='clearfix attachment__container'
                    style={{borderLeftColor: '#00832d'}}
                >
                    <h5 className='mt-1'>
                        {'Google Meet Conference'}
                    </h5>
                    <div>{post.message}</div>
                    {meetURL && (
                        <div style={{marginTop: '8px'}}>
                            <ExternalLink href={meetURL}>
                                {meetURL}
                            </ExternalLink>
                        </div>
                    )}
                    {spaceID && (
                        <div style={{marginTop: '4px', fontSize: '12px', color: 'rgba(var(--center-channel-color-rgb), 0.56)'}}>
                            {'Space: '}{spaceID}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};

export default PostTypeConference;
