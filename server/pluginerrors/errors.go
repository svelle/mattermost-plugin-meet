package pluginerrors

import "errors"

// ErrNeedsReconnect indicates the user must re-connect their Google account.
var ErrNeedsReconnect = errors.New("needs_reconnect")
