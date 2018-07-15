package webrtc

import (
	"testing"
	"time"
)

func TestRTCCertificate(t *testing.T) {
	t.Run("Equals", func(t *testing.T) {
		now := time.Now()
		testCases := []struct {
			first    RTCCertificate
			second   RTCCertificate
			expected bool
		}{
			{RTCCertificate{expires: now}, RTCCertificate{expires: now}, true},
			{RTCCertificate{expires: now}, RTCCertificate{}, false},
		}

		for i, testCase := range testCases {
			equal := testCase.first.Equals(testCase.second)
			if equal != testCase.expected {
				t.Errorf("Case %d: expected %t got %t", i, testCase.expected, equal)
			}
		}
	})
}
