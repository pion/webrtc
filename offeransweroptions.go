// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

// OfferAnswerOptions is a base structure which describes the options that
// can be used to control the offer/answer creation process.
type OfferAnswerOptions struct {
	// VoiceActivityDetection allows the application to provide information
	// about whether it wishes voice detection feature to be enabled or disabled.
	VoiceActivityDetection bool
	// ICETricklingSupported indicates whether the ICE agent should use trickle ICE
	// If set, the "a=ice-options:trickle" attribute is added to the generated SDP payload.
	// (See https://datatracker.ietf.org/doc/html/rfc9725#section-4.3.3)
	ICETricklingSupported bool
}

// AnswerOptions structure describes the options used to control the answer
// creation process.
type AnswerOptions struct {
	OfferAnswerOptions
}

// OfferOptions structure describes the options used to control the offer
// creation process.
type OfferOptions struct {
	OfferAnswerOptions

	// ICERestart forces the underlying ice gathering process to be restarted.
	// When this value is true, the generated description will have ICE
	// credentials that are different from the current credentials
	ICERestart bool
}
