// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestICECandidateInit_Serialization(t *testing.T) {
	tt := []struct {
		candidate  ICECandidateInit
		serialized string
	}{
		{ICECandidateInit{
			Candidate:        "candidate:abc123",
			SDPMid:           refString("0"),
			SDPMLineIndex:    refUint16(0),
			UsernameFragment: refString("def"),
		}, `{"candidate":"candidate:abc123","sdpMid":"0","sdpMLineIndex":0,"usernameFragment":"def"}`},
		{ICECandidateInit{
			Candidate: "candidate:abc123",
		}, `{"candidate":"candidate:abc123","sdpMid":null,"sdpMLineIndex":null,"usernameFragment":null}`},
	}

	for i, tc := range tt {
		b, err := json.Marshal(tc.candidate)
		if err != nil {
			t.Errorf("Failed to marshal %d: %v", i, err)
		}
		actualSerialized := string(b)
		if actualSerialized != tc.serialized {
			t.Errorf("%d expected %s got %s", i, tc.serialized, actualSerialized)
		}

		var actual ICECandidateInit
		err = json.Unmarshal(b, &actual)
		if err != nil {
			t.Errorf("Failed to unmarshal %d: %v", i, err)
		}

		assert.Equal(t, tc.candidate, actual, "should match")
	}
}

func refString(s string) *string {
	return &s
}

func refUint16(i uint16) *uint16 {
	return &i
}
