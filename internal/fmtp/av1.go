// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package fmtp

type av1FMTP struct {
	clockRate  uint32
	parameters map[string]string
}

func (h *av1FMTP) MimeType() string {
	return "video/av1"
}

func (h *av1FMTP) Match(b FMTP) bool {
	other, ok := b.(*av1FMTP)
	if !ok {
		return false
	}

	if !ClockRateEqual(h.MimeType(), h.clockRate, other.clockRate) {
		return false
	}

	// RTP Payload Format For AV1 (v1.0)
	// https://aomediacodec.github.io/av1-rtp-spec/
	// If the profile parameter is not present, it MUST be inferred to be 0 (“Main” profile).
	hProfile, ok := h.parameters["profile"]
	if !ok {
		hProfile = "0"
	}
	cProfile, ok := other.parameters["profile"]
	if !ok {
		cProfile = "0"
	}
	if hProfile != cProfile {
		return false
	}

	return true
}

func (h *av1FMTP) Parameter(key string) (string, bool) {
	v, ok := h.parameters[key]

	return v, ok
}
