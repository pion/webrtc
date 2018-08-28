package webrtc

import (
	"errors"
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
