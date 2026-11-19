// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package fmtp

import (
	"encoding/hex"
)

func profileLevelIDMatches(a, b string) bool {
	aa, err := hex.DecodeString(a)
	if err != nil || len(aa) < 2 {
		return false
	}
	bb, err := hex.DecodeString(b)
	if err != nil || len(bb) < 2 {
		return false
	}

	return h264Profile(aa) == h264Profile(bb)
}

// RFC 6184, Table 5 permits different profile_idc/profile-iop combinations
// to describe the same sub-profile. The level remains independent.
func h264Profile(profile []byte) uint16 {
	switch {
	case profile[0] == 0x42 && profile[1]&0x4f == 0x40,
		profile[0] == 0x4d && profile[1]&0x8f == 0x80,
		profile[0] == 0x58 && profile[1]&0xcf == 0xc0:
		return 0x42e0 // Constrained Baseline
	case profile[0] == 0x42 && profile[1]&0x4f == 0,
		profile[0] == 0x58 && profile[1]&0xcf == 0x80:
		return 0x4200 // Baseline
	case profile[0] == 0x4d && profile[1]&0xaf == 0:
		return 0x4d00 // Main
	case profile[0] == 0x58 && profile[1]&0xcf == 0:
		return 0x5800 // Extended
	default:
		return uint16(profile[0])<<8 | uint16(profile[1])
	}
}

type h264FMTP struct {
	parameters map[string]string
}

func (h *h264FMTP) MimeType() string {
	return "video/h264"
}

// Match returns true if h and b are compatible fmtp descriptions
// Based on RFC6184 Section 8.2.2:
//
//	The parameters identifying a media format configuration for H.264
//	are profile-level-id and packetization-mode.  These media format
//	configuration parameters (except for the level part of profile-
//	level-id) MUST be used symmetrically; that is, the answerer MUST
//	either maintain all configuration parameters or remove the media
//	format (payload type) completely if one or more of the parameter
//	values are not supported.
//	  Informative note: The requirement for symmetric use does not
//	  apply for the level part of profile-level-id and does not apply
//	  for the other stream properties and capability parameters.
func (h *h264FMTP) Match(b FMTP) bool {
	fmtp, ok := b.(*h264FMTP)
	if !ok {
		return false
	}

	// test packetization-mode
	hpmode, hok := h.parameters["packetization-mode"]
	if !hok {
		return false
	}
	cpmode, cok := fmtp.parameters["packetization-mode"]
	if !cok {
		return false
	}

	if hpmode != cpmode {
		return false
	}

	// test profile-level-id
	hplid, hok := h.parameters["profile-level-id"]
	if !hok {
		return false
	}

	cplid, cok := fmtp.parameters["profile-level-id"]
	if !cok {
		return false
	}

	if !profileLevelIDMatches(hplid, cplid) {
		return false
	}

	return true
}

func (h *h264FMTP) Parameter(key string) (string, bool) {
	v, ok := h.parameters[key]

	return v, ok
}
