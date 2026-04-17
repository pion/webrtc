// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
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

// UpdateWithCandidate adds a candidate to the SDP.
func (sd *SessionDescription) UpdateWithCandidate(candidate ICECandidateInit) error { //nolint:cyclop
	// Work on a fresh parse rather than the cached sd.parsed: that structure
	// may be concurrently read by the operations queue (e.g. dtlsRoleFromSDP,
	// startRTP) so mutating its MediaDescriptions would race.
	parsed := &sdp.SessionDescription{}
	if err := parsed.UnmarshalString(sd.SDP); err != nil {
		return fmt.Errorf("%w: %w", ErrSDPUnmarshalling, err)
	}

	var targetMedia *sdp.MediaDescription
	if candidate.SDPMid != nil {
		for _, m := range parsed.MediaDescriptions {
			if mid, ok := m.Attribute(sdp.AttrKeyMID); ok && mid == *candidate.SDPMid {
				targetMedia = m

				break
			}
		}
	} else if candidate.SDPMLineIndex != nil {
		if int(*candidate.SDPMLineIndex) < len(parsed.MediaDescriptions) {
			targetMedia = parsed.MediaDescriptions[*candidate.SDPMLineIndex]
		}
	}

	if targetMedia == nil {
		return nil
	}

	candidateValue := strings.TrimPrefix(candidate.Candidate, "candidate:")
	targetMedia.WithValueAttribute("candidate", candidateValue)

	marshaled, err := parsed.Marshal()
	if err != nil {
		return err
	}
	sd.SDP = string(marshaled)

	return nil
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
