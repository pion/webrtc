package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewICETransportPolicy(t *testing.T) {
	testCases := []struct {
		policyString   string
		expectedPolicy ICETransportPolicy
	}{
		{unknownStr, ICETransportPolicy(Unknown)},
		{"relay", ICETransportPolicyRelay},
		{"all", ICETransportPolicyAll},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedPolicy,
			newICETransportPolicy(testCase.policyString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestICETransportPolicy_String(t *testing.T) {
	testCases := []struct {
		policy         ICETransportPolicy
		expectedString string
	}{
		{ICETransportPolicy(Unknown), unknownStr},
		{ICETransportPolicyRelay, "relay"},
		{ICETransportPolicyAll, "all"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.policy.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
