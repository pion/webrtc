package ice

import (
	"fmt"
	"github.com/pkg/errors"
)

// Types of InvalidStateErrors
var (
	// ErrUnknownType indicates a Unknown info
	ErrUnknownType = errors.New("Unknown")

	// ErrServerType indicates the scheme type could not be parsed
	ErrSchemeType = errors.New("unknown scheme type")

	// ErrSTUNQuery indicates query arguments are provided in a STUN URL
	ErrSTUNQuery = errors.New("queries not supported in stun address")

	// ErrInvalidQuery indicates an malformed query is provided
	ErrInvalidQuery = errors.New("invalid query")

	// ErrHost indicates malformed hostname is provided
	ErrHost = errors.New("invalid hostname")

	// ErrPort indicates malformed port is provided
	ErrPort = errors.New("invalid port")

	// ErrProtoType indicates an unsupported transport type was provided
	ErrProtoType = errors.New("invalid transport protocol type")
)

// SyntaxError indicates the string did not match the expected pattern.
type SyntaxError struct {
	Err error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("ice: SyntaxError: %#v", e.Err)
}

// UnknownError indicates the operation failed for an unknown transient reason
type UnknownError struct {
	Err error
}

func (e *UnknownError) Error() string {
	return fmt.Sprintf("ice: UnknownError: %v", e.Err)
}

// NotSupportedError indicates the operation is not supported.
type NotSupportedError struct {
	Err error
}

func (e *NotSupportedError) Error() string {
	return fmt.Sprintf("ice: NotSupportedError: %v", e.Err)
}
