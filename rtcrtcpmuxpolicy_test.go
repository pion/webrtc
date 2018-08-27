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
			NewRTCRtcpMuxPolicy(testCase.policyString),
			testCase.expectedPolicy,
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
			testCase.policy.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
