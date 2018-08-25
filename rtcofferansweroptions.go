package webrtc

type RTCOfferAnswerOptions struct {
	VoiceActivityDetection bool
}

// RTCAnswerOptions describes the options used to control the answer creation process
type RTCAnswerOptions struct {
	RTCOfferAnswerOptions
}

// RTCOfferOptions describes the options used to control the offer creation process
type RTCOfferOptions struct {
	RTCOfferAnswerOptions
	ICERestart             bool
}