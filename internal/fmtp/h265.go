// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package fmtp

import "strings"

type h265FMTP struct {
	parameters map[string]string
}

func (h *h265FMTP) MimeType() string {
	return "video/h265"
}

// Match returns true if h and b describe compatible H.265 media formats.
// draft-ietf-avtcore-hevc-webrtc-09 Section 2.1 defines profile-id,
// tier-flag, and tx-mode as the H.265 media format parameters for WebRTC:
// https://datatracker.ietf.org/doc/html/draft-ietf-avtcore-hevc-webrtc-09#section-2.1
//
// level-id is intentionally not compared, as required by W3C WebRTC candidate
// correction 52:
// https://www.w3.org/TR/webrtc/#codec-match-without-level-id
// RFC 7798 Section 7.2.2 permits an answer to lower it, so treating it as a
// symmetric parameter would reject a valid offer/answer exchange:
// https://www.rfc-editor.org/rfc/rfc7798.html#section-7.2.2
func (h *h265FMTP) Match(b FMTP) bool {
	fmtp, ok := b.(*h265FMTP)
	if !ok {
		return false
	}

	return strings.EqualFold(
		h.parameterOrDefault("profile-id", "1"),
		fmtp.parameterOrDefault("profile-id", "1"),
	) && strings.EqualFold(
		h.parameterOrDefault("tier-flag", "0"),
		fmtp.parameterOrDefault("tier-flag", "0"),
	) && strings.EqualFold(
		h.parameterOrDefault("tx-mode", "SRST"),
		fmtp.parameterOrDefault("tx-mode", "SRST"),
	)
}

func (h *h265FMTP) Parameter(key string) (string, bool) {
	v, ok := h.parameters[key]

	return v, ok
}

func (h *h265FMTP) parameterOrDefault(key, defaultValue string) string {
	if value, ok := h.Parameter(key); ok {
		return value
	}

	return defaultValue
}
