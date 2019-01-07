package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCBundlePolicy(t *testing.T) {
	testCases := []struct {
		policyString   string
		expectedPolicy RTCBundlePolicy
	}{
		{unknownStr, RTCBundlePolicy(Unknown)},
		{"balanced", RTCBundlePolicyBalanced},
		{"max-compat", RTCBundlePolicyMaxCompat},
		{"max-bundle", RTCBundlePolicyMaxBundle},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedPolicy,
			newRTCBundlePolicy(testCase.policyString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCBundlePolicy_String(t *testing.T) {
	testCases := []struct {
		policy         RTCBundlePolicy
		expectedString string
	}{
		{RTCBundlePolicy(Unknown), unknownStr},
		{RTCBundlePolicyBalanced, "balanced"},
		{RTCBundlePolicyMaxCompat, "max-compat"},
		{RTCBundlePolicyMaxBundle, "max-bundle"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.policy.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
