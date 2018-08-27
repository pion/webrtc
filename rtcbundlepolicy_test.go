package webrtc

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewRTCBundlePolicy(t *testing.T) {
	testCases := []struct {
		policyString   string
		expectedPolicy RTCBundlePolicy
	}{
		{"unknown", RTCBundlePolicy(Unknown)},
		{"balanced", RTCBundlePolicyBalanced},
		{"max-compat", RTCBundlePolicyMaxCompat},
		{"max-bundle", RTCBundlePolicyMaxBundle},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			NewRTCBundlePolicy(testCase.policyString),
			testCase.expectedPolicy,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCBundlePolicy_String(t *testing.T) {
	testCases := []struct {
		policy         RTCBundlePolicy
		expectedString string
	}{
		{RTCBundlePolicy(Unknown), "unknown"},
		{RTCBundlePolicyBalanced, "balanced"},
		{RTCBundlePolicyMaxCompat, "max-compat"},
		{RTCBundlePolicyMaxBundle, "max-bundle"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.policy.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
