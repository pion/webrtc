package webrtc

// RTPReceiveParameters contains the RTP stack settings used by receivers
type RTPReceiveParameters struct {
	RTPParameters
	Encodings []RTPDecodingParameters
}
