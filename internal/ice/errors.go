package ice

import "errors"

var (
	// ErrUnknownType indicates an error with Unknown info.
	ErrUnknownType = errors.New("unknown")

	// ErrNoTurnCredencials indicates that a TURN server URL was provided
	// without required credentials.
	ErrNoTurnCredencials = errors.New("turn server credentials required")

	// ErrTurnCredencials indicates that provided TURN credentials are partial
	// or malformed.
	ErrTurnCredencials = errors.New("invalid turn server credentials")
)
