package webrtc

import (
	"errors"
	"fmt"
)

var (
	// ErrUnknownType indicates an error with Unknown info.
	ErrUnknownType = errors.New("unknown")

	// ErrConnectionClosed indicates an operation executed after connection
	// has already been closed.
	ErrConnectionClosed = errors.New("connection closed")

	// ErrCertificateExpired indicates that an x509 certificate has expired.
	ErrCertificateExpired = errors.New("x509Cert expired")

	// ErrNoTurnCredencials indicates that a TURN server URL was provided
	// without required credentials.
	ErrNoTurnCredencials = errors.New("turn server credentials required")

	// ErrTurnCredencials indicates that provided TURN credentials are partial
	// or malformed.
	ErrTurnCredencials = errors.New("invalid turn server credentials")

	// ErrExistingTrack indicates that a track already exists.
	ErrExistingTrack = errors.New("track aready exists")

	// ErrPrivateKeyType indicates that a particular private key encryption
	// chosen to generate a certificate is not supported.
	ErrPrivateKeyType = errors.New("private key type not supported")

	// ErrModifyingPeerIdentity indicates that an attempt to modify
	// PeerIdentity was made after RTCPeerConnection has been initialized.
	ErrModifyingPeerIdentity = errors.New("peerIdentity cannot be modified")

	// ErrModifyingCertificates indicates that an attempt to modify
	// Certificates was made after RTCPeerConnection has been initialized.
	ErrModifyingCertificates = errors.New("certificates cannot be modified")

	// ErrModifyingBundlePolicy indicates that an attempt to modify
	// BundlePolicy was made after RTCPeerConnection has been initialized.
	ErrModifyingBundlePolicy = errors.New("bundle policy cannot be modified")

	// ErrModifyingRtcpMuxPolicy indicates that an attempt to modify
	// RtcpMuxPolicy was made after RTCPeerConnection has been initialized.
	ErrModifyingRtcpMuxPolicy = errors.New("rtcp mux policy cannot be modified")

	// ErrModifyingIceCandidatePoolSize indicates that an attempt to modify
	// IceCandidatePoolSize was made after RTCPeerConnection has been initialized.
	ErrModifyingIceCandidatePoolSize = errors.New("ice candidate pool size cannot be modified")

	// ErrInvalidValue indicates that an invalid value was provided.
	ErrInvalidValue = errors.New("invalid value")

	// ErrMaxDataChannels indicates that the maximum number of data channels
	// was reached.
	ErrMaxDataChannels = errors.New("maximum number of datachannels reached")
)

// InvalidStateError indicates the object is in an invalid state.
type InvalidStateError struct {
	Err error
}

func (e *InvalidStateError) Error() string {
	return fmt.Sprintf("webrtc: InvalidStateError: %v", e.Err)
}

// UnknownError indicates the operation failed for an unknown transient reason
type UnknownError struct {
	Err error
}

func (e *UnknownError) Error() string {
	return fmt.Sprintf("webrtc: UnknownError: %v", e.Err)
}

// InvalidAccessError indicates the object does not support the operation or argument.
type InvalidAccessError struct {
	Err error
}

func (e *InvalidAccessError) Error() string {
	return fmt.Sprintf("webrtc: InvalidAccessError: %v", e.Err)
}

// NotSupportedError indicates the operation is not supported.
type NotSupportedError struct {
	Err error
}

func (e *NotSupportedError) Error() string {
	return fmt.Sprintf("webrtc: NotSupportedError: %v", e.Err)
}

// InvalidModificationError indicates the object can not be modified in this way.
type InvalidModificationError struct {
	Err error
}

func (e *InvalidModificationError) Error() string {
	return fmt.Sprintf("webrtc: InvalidModificationError: %v", e.Err)
}

// SyntaxError indicates the string did not match the expected pattern.
type SyntaxError struct {
	Err error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("webrtc: SyntaxError: %v", e.Err)
}

// TypeError indicates an issue with a supplied value
type TypeError struct {
	Err error
}

func (e *TypeError) Error() string {
	return fmt.Sprintf("webrtc: TypeError: %v", e.Err)
}

// OperationError indicates an issue with execution
type OperationError struct {
	Err error
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("webrtc: OperationError: %v", e.Err)
}
