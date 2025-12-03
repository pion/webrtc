// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pion/sdp/v3"
)

// ICETrickleCapability represents whether the remote endpoint accepts
// trickled ICE candidates.
type ICETrickleCapability int

const (
	// ICETrickleCapabilityUnknown no remote peer has been established.
	ICETrickleCapabilityUnknown ICETrickleCapability = iota
	// ICETrickleCapabilitySupported remote peer can accept trickled ICE candidates.
	ICETrickleCapabilitySupported
	// ICETrickleCapabilitySupported remote peer didn't state that it can accept trickle ICE candidates.
	ICETrickleCapabilityUnsupported
)

// String returns the string representation of ICETrickleCapability.
func (t ICETrickleCapability) String() string {
	switch t {
	case ICETrickleCapabilitySupported:
		return "supported"
	case ICETrickleCapabilityUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

// SessionDescription is used to expose local and remote session descriptions.
type SessionDescription struct {
	Type SDPType `json:"type"`
	SDP  string  `json:"sdp"`

	// This will never be initialized by callers, internal use only
	parsed *sdp.SessionDescription
}

// Unmarshal is a helper to deserialize the sdp.
func (sd *SessionDescription) Unmarshal() (*sdp.SessionDescription, error) {
	sd.parsed = &sdp.SessionDescription{}
	err := sd.parsed.UnmarshalString(sd.SDP)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSDPUnmarshalling, err)
	}

	return sd.parsed, nil
}

func hasICETrickleOption(desc *sdp.SessionDescription) bool {
	if value, ok := desc.Attribute(sdp.AttrKeyICEOptions); ok && hasTrickleOptionValue(value) {
		return true
	}

	for _, media := range desc.MediaDescriptions {
		if value, ok := media.Attribute(sdp.AttrKeyICEOptions); ok && hasTrickleOptionValue(value) {
			return true
		}
	}

	return false
}

func hasTrickleOptionValue(value string) bool {
	return slices.Contains(strings.Fields(value), "trickle")
}
