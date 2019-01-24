package webrtc

// RTCRtpCodingParameters provides information relating to both encoding and decoding.
// This is a subset of the RFC since Pion WebRTC doesn't implement encoding/decoding itself
// http://draft.ortc.org/#dom-rtcrtpcodingparameters
type RTCRtpCodingParameters struct {
	SSRC        uint32 `json:"ssrc"`
	PayloadType uint8  `json:"payloadType"`
}
