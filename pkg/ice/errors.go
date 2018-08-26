package ice

import (
	"errors"
	"fmt"
)

// SyntaxError indicates the string did not match the expected pattern.
type SyntaxError struct {
	Err error
}

func (e SyntaxError) Error() string {
	return fmt.Sprintf("ice: SyntaxError: %v", e.Err)
}

// Types of InvalidStateErrors
var (
	// ErrServerType indicates the scheme type could not be parsed
	ErrSchemeType = errors.New("unknown scheme type")

	// ErrSTUNQuery indicates query arguments are provided in a STUN URL
	ErrSTUNQuery = errors.New("queries not supported in stun address")

	// ErrInvalidQuery indicates an unsupported query is provided
	ErrInvalidQuery = errors.New("invalid query")

	// ErrProtoType indicates an unsupported transport type was provided
	ErrProtoType = errors.New("invalid transport protocol type")

	// ErrHost indicates the server hostname could not be parsed
	ErrHost = errors.New("invalid hostname")

	// ErrPort indicates the server port could not be parsed
	ErrPort = errors.New("invalid port")
)

// UnknownError indicates the operation failed for an unknown transient reason
type UnknownError struct {
	Err error
}

func (e UnknownError) Error() string {
	return fmt.Sprintf("ice: UnknownError: %v", e.Err)
}

// Types of UnknownErrors
var ()
