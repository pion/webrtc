// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

// SCTPCapabilities indicates the capabilities of the SCTPTransport.
type SCTPCapabilities struct {
	MaxMessageSize uint32 `json:"maxMessageSize"`
	// Note: this is the binary sctp-init, not the base64 encoded version.
	SctpInit string `json:"sctpInit"`
}
