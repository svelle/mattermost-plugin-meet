/* eslint-disable react/prop-types */
import React from 'react';

type Props = React.AnchorHTMLAttributes<HTMLAnchorElement>;

const ExternalLink = ({children, rel, target, ...props}: Props) => {
    return (
        <a
            {...props}
            rel={rel || 'noopener noreferrer'}
            target={target || '_blank'}
        >
            {children}
        </a>
    );
};

export default ExternalLink;
