// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBundlePolicy(t *testing.T) {
	testCases := []struct {
		policyString   string
		expectedPolicy BundlePolicy
	}{
		{unknownStr, BundlePolicy(Unknown)},
		{"balanced", BundlePolicyBalanced},
		{"max-compat", BundlePolicyMaxCompat},
		{"max-bundle", BundlePolicyMaxBundle},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedPolicy,
			newBundlePolicy(testCase.policyString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestBundlePolicy_String(t *testing.T) {
	testCases := []struct {
		policy         BundlePolicy
		expectedString string
	}{
		{BundlePolicy(Unknown), unknownStr},
		{BundlePolicyBalanced, "balanced"},
		{BundlePolicyMaxCompat, "max-compat"},
		{BundlePolicyMaxBundle, "max-bundle"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.policy.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
