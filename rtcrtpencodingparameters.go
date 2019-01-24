package webrtc

// RTCRtpEncodingParameters provides information relating to both encoding and decoding.
// This is a subset of the RFC since Pion WebRTC doesn't implement encoding itself
// http://draft.ortc.org/#dom-rtcrtpencodingparameters
type RTCRtpEncodingParameters struct {
	RTCRtpCodingParameters
}
