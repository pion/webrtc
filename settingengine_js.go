// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build js && wasm
// +build js,wasm

package webrtc

// SettingEngine allows influencing behavior in ways that are not
// supported by the WebRTC API. This allows us to support additional
// use-cases without deviating from the WebRTC API elsewhere.
type SettingEngine struct {
	detach struct {
		DataChannels bool
	}

	disableRTPHeaderEncryption bool
}

// DetachDataChannels enables detaching data channels. When enabled
// data channels have to be detached in the OnOpen callback using the
// DataChannel.Detach method.
func (e *SettingEngine) DetachDataChannels() {
	e.detach.DataChannels = true
}

// DisableRTPHeaderEncryption disables RFC 9335 Cryptex negotiation.
// When isDisabled is true, the PeerConnection will not advertise Cryptex in generated SDP.
func (e *SettingEngine) DisableRTPHeaderEncryption(isDisabled bool) {
	e.disableRTPHeaderEncryption = isDisabled
}
