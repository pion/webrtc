// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

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
		{"nohost", ICETransportPolicyNoHost},
		{"relay", ICETransportPolicyRelay},
		{"all", ICETransportPolicyAll},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedPolicy,
			NewICETransportPolicy(testCase.policyString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestICETransportPolicy_String(t *testing.T) {
	testCases := []struct {
		policy         ICETransportPolicy
		expectedString string
	}{
		{ICETransportPolicyNoHost, "nohost"},
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
