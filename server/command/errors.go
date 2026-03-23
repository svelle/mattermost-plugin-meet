package command

import "errors"

// ErrNeedsReconnect indicates the user must re-connect their Google account.
var ErrNeedsReconnect = errors.New("needs_reconnect")

// ErrPublicChannelRestricted indicates meeting creation is blocked in public channels.
var ErrPublicChannelRestricted = errors.New("public_channel_restricted")
