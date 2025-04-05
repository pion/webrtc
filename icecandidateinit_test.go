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
		assert.NoErrorf(t, err, "test case %d", i)
		actualSerialized := string(b)
		assert.Equalf(t, tc.serialized, actualSerialized, "test case %d", i)

		var actual ICECandidateInit
		err = json.Unmarshal(b, &actual)
		assert.NoErrorf(t, err, "test case %d", i)
		assert.Equalf(t, tc.candidate, actual, "test case %d", i)
	}
}

func refString(s string) *string {
	return &s
}

func refUint16(i uint16) *uint16 {
	return &i
}
