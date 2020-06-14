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

	// ErrDataChannelNotOpen indicates an operation executed when the data
	// channel is not (yet) open.
	ErrDataChannelNotOpen = errors.New("data channel not open")

	// ErrCertificateExpired indicates that an x509 certificate has expired.
	ErrCertificateExpired = errors.New("x509Cert expired")

	// ErrNoTurnCredentials indicates that a TURN server URL was provided
	// without required credentials.
	ErrNoTurnCredentials = errors.New("turn server credentials required")

	// ErrTurnCredentials indicates that provided TURN credentials are partial
	// or malformed.
	ErrTurnCredentials = errors.New("invalid turn server credentials")

	// ErrExistingTrack indicates that a track already exists.
	ErrExistingTrack = errors.New("track already exists")

	// ErrPrivateKeyType indicates that a particular private key encryption
	// chosen to generate a certificate is not supported.
	ErrPrivateKeyType = errors.New("private key type not supported")

	// ErrModifyingPeerIdentity indicates that an attempt to modify
	// PeerIdentity was made after PeerConnection has been initialized.
	ErrModifyingPeerIdentity = errors.New("peerIdentity cannot be modified")

	// ErrModifyingCertificates indicates that an attempt to modify
	// Certificates was made after PeerConnection has been initialized.
	ErrModifyingCertificates = errors.New("certificates cannot be modified")

	// ErrModifyingBundlePolicy indicates that an attempt to modify
	// BundlePolicy was made after PeerConnection has been initialized.
	ErrModifyingBundlePolicy = errors.New("bundle policy cannot be modified")

	// ErrModifyingRTCPMuxPolicy indicates that an attempt to modify
	// RTCPMuxPolicy was made after PeerConnection has been initialized.
	ErrModifyingRTCPMuxPolicy = errors.New("rtcp mux policy cannot be modified")

	// ErrModifyingICECandidatePoolSize indicates that an attempt to modify
	// ICECandidatePoolSize was made after PeerConnection has been initialized.
	ErrModifyingICECandidatePoolSize = errors.New("ice candidate pool size cannot be modified")

	// ErrStringSizeLimit indicates that the character size limit of string is
	// exceeded. The limit is hardcoded to 65535 according to specifications.
	ErrStringSizeLimit = errors.New("data channel label exceeds size limit")

	// ErrMaxDataChannelID indicates that the maximum number ID that could be
	// specified for a data channel has been exceeded.
	ErrMaxDataChannelID = errors.New("maximum number ID for datachannel specified")

	// ErrNegotiatedWithoutID indicates that an attempt to create a data channel
	// was made while setting the negotiated option to true without providing
	// the negotiated channel ID.
	ErrNegotiatedWithoutID = errors.New("negotiated set without channel id")

	// ErrRetransmitsOrPacketLifeTime indicates that an attempt to create a data
	// channel was made with both options MaxPacketLifeTime and MaxRetransmits
	// set together. Such configuration is not supported by the specification
	// and is mutually exclusive.
	ErrRetransmitsOrPacketLifeTime = errors.New("both MaxPacketLifeTime and MaxRetransmits was set")

	// ErrCodecNotFound is returned when a codec search to the Media Engine fails
	ErrCodecNotFound = errors.New("codec not found")

	// ErrNoRemoteDescription indicates that an operation was rejected because
	// the remote description is not set
	ErrNoRemoteDescription = errors.New("remote description is not set")

	// ErrIncorrectSDPSemantics indicates that the PeerConnection was configured to
	// generate SDP Answers with different SDP Semantics than the received Offer
	ErrIncorrectSDPSemantics = errors.New("offer SDP semantics does not match configuration")

	// ErrProtocolTooLarge indicates that value given for a DataChannelInit protocol is
	//longer then 65535 bytes
	ErrProtocolTooLarge = errors.New("protocol is larger then 65535 bytes")

	// ErrSenderNotCreatedByConnection indicates RemoveTrack was called with a RtpSender not created
	// by this PeerConnection
	ErrSenderNotCreatedByConnection = errors.New("RtpSender not created by this PeerConnection")

	// ErrSessionDescriptionNoFingerprint indicates SetRemoteDescription was called with a SessionDescription that has no
	// fingerprint
	ErrSessionDescriptionNoFingerprint = errors.New("SetRemoteDescription called with no fingerprint")

	// ErrSessionDescriptionInvalidFingerprint indicates SetRemoteDescription was called with a SessionDescription that
	// has an invalid fingerprint
	ErrSessionDescriptionInvalidFingerprint = errors.New("SetRemoteDescription called with an invalid fingerprint")

	// ErrSessionDescriptionConflictingFingerprints indicates SetRemoteDescription was called with a SessionDescription that
	// has an conflicting fingerprints
	ErrSessionDescriptionConflictingFingerprints = errors.New("SetRemoteDescription called with multiple conflicting fingerprint")

	// ErrSessionDescriptionMissingIceUfrag indicates SetRemoteDescription was called with a SessionDescription that
	// is missing an ice-ufrag value
	ErrSessionDescriptionMissingIceUfrag = errors.New("SetRemoteDescription called with no ice-ufrag")

	// ErrSessionDescriptionMissingIcePwd indicates SetRemoteDescription was called with a SessionDescription that
	// is missing an ice-pwd value
	ErrSessionDescriptionMissingIcePwd = errors.New("SetRemoteDescription called with no ice-pwd")

	// ErrSessionDescriptionConflictingIceUfrag  indicates SetRemoteDescription was called with a SessionDescription that
	// contains multiple conflicting ice-ufrag values
	ErrSessionDescriptionConflictingIceUfrag = errors.New("SetRemoteDescription called with multiple conflicting ice-ufrag values")

	// ErrSessionDescriptionConflictingIcePwd indicates SetRemoteDescription was called with a SessionDescription that
	// contains multiple conflicting ice-pwd values
	ErrSessionDescriptionConflictingIcePwd = errors.New("SetRemoteDescription called with multiple conflicting ice-pwd values")
)
