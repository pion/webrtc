// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCPMuxPolicy(t *testing.T) {
	testCases := []struct {
		policyString   string
		expectedPolicy RTCPMuxPolicy
	}{
		{ErrUnknownType.Error(), RTCPMuxPolicyUnknown},
		{"negotiate", RTCPMuxPolicyNegotiate},
		{"require", RTCPMuxPolicyRequire},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedPolicy,
			newRTCPMuxPolicy(testCase.policyString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCPMuxPolicy_String(t *testing.T) {
	testCases := []struct {
		policy         RTCPMuxPolicy
		expectedString string
	}{
		{RTCPMuxPolicyUnknown, ErrUnknownType.Error()},
		{RTCPMuxPolicyNegotiate, "negotiate"},
		{RTCPMuxPolicyRequire, "require"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.policy.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
