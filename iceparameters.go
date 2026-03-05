// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

// ICEParameters includes the ICE username fragment
// and password and other ICE-related parameters.
type ICEParameters struct {
	UsernameFragment string `json:"usernameFragment"`
	Password         string `json:"password"` //nolint:gosec // not a secret.
	ICELite          bool   `json:"iceLite"`
}
