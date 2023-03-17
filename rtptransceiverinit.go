package webrtc

// RTPTransceiverInit dictionary is used when calling the WebRTC function addTransceiver() to provide configuration options for the new transceiver.
type RTPTransceiverInit struct {
	Direction     RTPTransceiverDirection
	SendEncodings []RTPEncodingParameters
	SSRCOverride  SSRC
	// Streams       []*Track
}

// RtpTransceiverInit is a temporary mapping while we fix case sensitivity
// Deprecated: Use RTPTransceiverInit instead
type RtpTransceiverInit = RTPTransceiverInit //nolint: stylecheck,golint
