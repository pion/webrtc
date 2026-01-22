// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

// RTPReceiveParameters contains the RTP stack settings used by receivers.
type RTPReceiveParameters struct {
	Encodings []RTPDecodingParameters
}
