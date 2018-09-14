package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCRtcpMuxPolicy(t *testing.T) {
	testCases := []struct {
		policyString   string
		expectedPolicy RTCRtcpMuxPolicy
	}{
		{"unknown", RTCRtcpMuxPolicy(Unknown)},
		{"negotiate", RTCRtcpMuxPolicyNegotiate},
		{"require", RTCRtcpMuxPolicyRequire},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedPolicy,
			newRTCRtcpMuxPolicy(testCase.policyString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCRtcpMuxPolicy_String(t *testing.T) {
	testCases := []struct {
		policy         RTCRtcpMuxPolicy
		expectedString string
	}{
		{RTCRtcpMuxPolicy(Unknown), "unknown"},
		{RTCRtcpMuxPolicyNegotiate, "negotiate"},
		{RTCRtcpMuxPolicyRequire, "require"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.policy.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
