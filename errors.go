package webrtc

import (
	"errors"
	"fmt"
)

// InvalidStateError indicates the object is in an invalid state.
type InvalidStateError struct {
	Err error
}

func (e *InvalidStateError) Error() string {
	return fmt.Sprintf("webrtc: InvalidStateError: %v", e.Err)
}

// Types of InvalidStateErrors
var (
	ErrConnectionClosed = errors.New("connection closed")
)

// UnknownError indicates the operation failed for an unknown transient reason
type UnknownError struct {
	Err error
}

func (e *UnknownError) Error() string {
	return fmt.Sprintf("webrtc: UnknownError: %v", e.Err)
}

// Types of UnknownErrors
var (
	ErrNoConfig = errors.New("no configuration provided")
)

// InvalidAccessError indicates the object does not support the operation or argument.
type InvalidAccessError struct {
	Err error
}

func (e *InvalidAccessError) Error() string {
	return fmt.Sprintf("webrtc: InvalidAccessError: %v", e.Err)
}

// Types of InvalidAccessErrors
var (
	ErrCertificateExpired = errors.New("certificate expired")
	ErrNoTurnCred         = errors.New("turn server credentials required")
	ErrTurnCred           = errors.New("invalid turn server credentials")
	ErrExistingTrack      = errors.New("track aready exists")
)

// NotSupportedError indicates the operation is not supported.
type NotSupportedError struct {
	Err error
}

func (e *NotSupportedError) Error() string {
	return fmt.Sprintf("webrtc: NotSupportedError: %v", e.Err)
}

// Types of NotSupportedErrors
var ()

// InvalidModificationError indicates the object can not be modified in this way.
type InvalidModificationError struct {
	Err error
}

func (e *InvalidModificationError) Error() string {
	return fmt.Sprintf("webrtc: InvalidModificationError: %v", e.Err)
}

// Types of InvalidModificationErrors
var (
	ErrModifyingPeerIdentity         = errors.New("peerIdentity cannot be modified")
	ErrModifyingCertificates         = errors.New("certificates cannot be modified")
	ErrModifyingBundlePolicy         = errors.New("bundle policy cannot be modified")
	ErrModifyingRtcpMuxPolicy        = errors.New("rtcp mux policy cannot be modified")
	ErrModifyingIceCandidatePoolSize = errors.New("ice candidate pool size cannot be modified")
)

// SyntaxError indicates the string did not match the expected pattern.
type SyntaxError struct {
	Err error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("webrtc: SyntaxError: %v", e.Err)
}

// Types of SyntaxErrors
var ()

// TypeError indicates an issue with a supplied value
type TypeError struct {
	Err error
}

func (e *TypeError) Error() string {
	return fmt.Sprintf("webrtc: TypeError: %v", e.Err)
}

// Types of TypeError
var (
	ErrInvalidValue = errors.New("invalid value")
)

// OperationError indicates an issue with execution
type OperationError struct {
	Err error
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("webrtc: OperationError: %v", e.Err)
}

// Types of OperationError
var (
	ErrMaxDataChannels = errors.New("maximum number of datachannels reached")
)

// ErrUnknownType indicates a Unknown info
var ErrUnknownType = errors.New("Unknown")
