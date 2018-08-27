package webrtc

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
			NewRTCIceTransportPolicy(testCase.policyString),
			testCase.expectedPolicy,
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
			testCase.policy.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
