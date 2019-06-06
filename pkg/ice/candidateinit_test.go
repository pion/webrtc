package ice

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCandidateInit_Serialization(t *testing.T) {
	tt := []struct {
		candidate  CandidateInit
		serialized string
	}{
		{CandidateInit{
			Candidate:        "candidate:abc123",
			SDPMid:           refString("0"),
			SDPMLineIndex:    refUint16(0),
			UsernameFragment: "def",
		}, `{"candidate":"candidate:abc123","sdpMid":"0","sdpMLineIndex":0,"usernameFragment":"def"}`},
		{CandidateInit{
			Candidate:        "candidate:abc123",
			UsernameFragment: "def",
		}, `{"candidate":"candidate:abc123","usernameFragment":"def"}`},
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

		var actual CandidateInit
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
