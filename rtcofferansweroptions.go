package webrtc

// RTCOfferAnswerOptions is a base structure which describes the options that
// can be used to control the offer/answer creation process.
type RTCOfferAnswerOptions struct {
	// VoiceActivityDetection allows the application to provide information
	// about whether it wishes voice detection feature to be enabled or disabled.
	VoiceActivityDetection bool
}

// RTCAnswerOptions structure describes the options used to control the answer
// creation process.
type RTCAnswerOptions struct {
	RTCOfferAnswerOptions
}

// RTCOfferOptions structure describes the options used to control the offer
// creation process
type RTCOfferOptions struct {
	RTCOfferAnswerOptions

	// IceRestart forces the underlying ice gathering process to be restarted.
	// When this value is true, the generated description will have ICE
	// credentials that are different from the current credentials
	IceRestart bool
}
