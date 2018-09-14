package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCIceTransportPolicy(t *testing.T) {
	testCases := []struct {
		policyString   string
		expectedPolicy RTCIceTransportPolicy
	}{
		{"unknown", RTCIceTransportPolicy(Unknown)},
		{"relay", RTCIceTransportPolicyRelay},
		{"all", RTCIceTransportPolicyAll},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedPolicy,
			newRTCIceTransportPolicy(testCase.policyString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCIceTransportPolicy_String(t *testing.T) {
	testCases := []struct {
		policy         RTCIceTransportPolicy
		expectedString string
	}{
		{RTCIceTransportPolicy(Unknown), "unknown"},
		{RTCIceTransportPolicyRelay, "relay"},
		{RTCIceTransportPolicyAll, "all"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.policy.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
