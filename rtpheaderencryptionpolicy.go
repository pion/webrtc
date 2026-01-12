// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// nolint:dupl,staticcheck
package webrtc

import (
	"encoding/json"
)

// RTPHeaderEncryptionPolicy affects whether RTP header extension encryption is negotiated if the remote endpoint
// does not support RFC 9335. If the remote endpoint supports RFC 9335, all media streams are sent utilizing RFC 9335.
type RTPHeaderEncryptionPolicy int

const (
	// RTPHeaderEncryptionPolicyUnknown is the enum's zero-value.
	RTPHeaderEncryptionPolicyUnknown RTPHeaderEncryptionPolicy = iota

	// RTPHeaderEncryptionPolicyDisable indicates to disable RTP header extension encryption. This disables negotiation
	// of RTP header extension encryption as defined in RFC 9335 using the "cryptex" SDP attribute.
	RTPHeaderEncryptionPolicyDisable

	// RTPHeaderEncryptionPolicyNegotiate indicates to negotiate RTP header extension encryption as defined in RFC 9335.
	// If encryption cannot be negotiated, RTP header extensions are sent in the clear.
	RTPHeaderEncryptionPolicyNegotiate

	// RTPHeaderEncryptionPolicyRequire indicates to require RTP header extension encryption. If encryption cannot be
	// negotiated, session negotiation will fail.
	RTPHeaderEncryptionPolicyRequire
)

// This is done this way because of a linter.
const (
	rtpHeaderEncryptionPolicyDisable      = "disable"
	rtpHeaderEncryptionPolicyNegotiateStr = "negotiate"
	rtpHeaderEncryptionPolicyRequireStr   = "require"
)

func newRTPHeaderEncryptionPolicy(raw string) RTPHeaderEncryptionPolicy {
	switch raw {
	case rtpHeaderEncryptionPolicyDisable:
		return RTPHeaderEncryptionPolicyDisable
	case rtpHeaderEncryptionPolicyNegotiateStr:
		return RTPHeaderEncryptionPolicyNegotiate
	case rtpHeaderEncryptionPolicyRequireStr:
		return RTPHeaderEncryptionPolicyRequire
	default:
		return RTPHeaderEncryptionPolicyUnknown
	}
}

// String returns the string representation of the RTPHeaderEncryptionPolicy.
func (t RTPHeaderEncryptionPolicy) String() string {
	switch t {
	case RTPHeaderEncryptionPolicyDisable:
		return rtpHeaderEncryptionPolicyDisable
	case RTPHeaderEncryptionPolicyNegotiate:
		return rtpHeaderEncryptionPolicyNegotiateStr
	case RTPHeaderEncryptionPolicyRequire:
		return rtpHeaderEncryptionPolicyRequireStr
	default:
		return ErrUnknownType.Error()
	}
}

// UnmarshalJSON parses the JSON-encoded data and stores the result.
func (t *RTPHeaderEncryptionPolicy) UnmarshalJSON(b []byte) error {
	var val string
	if err := json.Unmarshal(b, &val); err != nil {
		return err
	}

	*t = newRTPHeaderEncryptionPolicy(val)

	return nil
}

// MarshalJSON returns the JSON encoding.
func (t RTPHeaderEncryptionPolicy) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}
