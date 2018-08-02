package webrtc

import (
	"errors"
	"fmt"
)

// Types of InvalidStateErrors
var (
	ErrConnectionClosed = errors.New("connection closed")
)

// InvalidStateError indicates the object is in an invalid state.
type InvalidStateError struct {
	Err error
}

func (e *InvalidStateError) Error() string {
	return fmt.Sprintf("invalid state error: %v", e.Err)
}

// Types of UnknownErrors
var (
	ErrNoConfig = errors.New("no configuration provided")
)

// UnknownError indicates the operation failed for an unknown transient reason
type UnknownError struct {
	Err error
}

func (e *UnknownError) Error() string {
	return fmt.Sprintf("unknown error: %v", e.Err)
}

// Types of InvalidAccessErrors
var (
	ErrCertificateExpired = errors.New("certificate expired")
	ErrNoTurnCred         = errors.New("turn server credentials required")
	ErrTurnCred           = errors.New("invalid turn server credentials")
	ErrExistingTrack      = errors.New("track aready exists")
)

// InvalidAccessError indicates the object does not support the operation or argument.
type InvalidAccessError struct {
	Err error
}

func (e *InvalidAccessError) Error() string {
	return fmt.Sprintf("invalid access error: %v", e.Err)
}

// Types of NotSupportedErrors
var ()

// NotSupportedError indicates the operation is not supported.
type NotSupportedError struct {
	Err error
}

func (e *NotSupportedError) Error() string {
	return fmt.Sprintf("not supported error: %v", e.Err)
}

// Types of InvalidModificationErrors
var (
	ErrModPeerIdentity         = errors.New("peer identity cannot be modified")
	ErrModCertificates         = errors.New("certificates cannot be modified")
	ErrModRtcpMuxPolicy        = errors.New("rtcp mux policy cannot be modified")
	ErrModICECandidatePoolSize = errors.New("ice candidate pool size cannot be modified")
)

// InvalidModificationError indicates the object can not be modified in this way.
type InvalidModificationError struct {
	Err error
}

func (e *InvalidModificationError) Error() string {
	return fmt.Sprintf("invalid modification error: %v", e.Err)
}

// Types of SyntaxErrors
var ()

// SyntaxError indicates the string did not match the expected pattern.
type SyntaxError struct {
	Err error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("syntax error: %v", e.Err)
}

// Types of TypeError
var (
	ErrInvalidValue = errors.New("invalid value")
)

// TypeError indicates an issue with a supplied value
type TypeError struct {
	Err error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("type error: %v", e.Err)
}

// Types of OperationError
var (
	ErrMaxDataChannels = errors.New("maximum number of datachannels reached")
)

// OperationError indicates an issue with execution
type OperationError struct {
	Err error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("operation error: %v", e.Err)
}
