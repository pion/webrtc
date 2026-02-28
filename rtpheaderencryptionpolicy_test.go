// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTPHeaderEncryptionPolicy(t *testing.T) {
	testCases := []struct {
		policyString   string
		expectedPolicy RTPHeaderEncryptionPolicy
	}{
		{ErrUnknownType.Error(), RTPHeaderEncryptionPolicyUnknown},
		{"disable", RTPHeaderEncryptionPolicyDisable},
		{"negotiate", RTPHeaderEncryptionPolicyNegotiate},
		{"require", RTPHeaderEncryptionPolicyRequire},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedPolicy,
			newRTPHeaderEncryptionPolicy(testCase.policyString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTPHeaderEncryptionPolicy_String(t *testing.T) {
	testCases := []struct {
		policy         RTPHeaderEncryptionPolicy
		expectedString string
	}{
		{RTPHeaderEncryptionPolicyUnknown, ErrUnknownType.Error()},
		{RTPHeaderEncryptionPolicyDisable, "disable"},
		{RTPHeaderEncryptionPolicyNegotiate, "negotiate"},
		{RTPHeaderEncryptionPolicyRequire, "require"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.policy.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTPHeaderEncryptionPolicy_MarshalUnmarshal(t *testing.T) {
	testCases := []RTPHeaderEncryptionPolicy{
		RTPHeaderEncryptionPolicyUnknown,
		RTPHeaderEncryptionPolicyDisable,
		RTPHeaderEncryptionPolicyNegotiate,
		RTPHeaderEncryptionPolicyRequire,
	}

	for i, testCase := range testCases {
		bytes, err := testCase.MarshalJSON()
		assert.NoError(t, err, "testCase: %d %v", i, testCase)

		var policy RTPHeaderEncryptionPolicy
		err = policy.UnmarshalJSON(bytes)
		assert.NoError(t, err, "testCase: %d %v", i, testCase)

		assert.Equal(t,
			testCase,
			policy,
			"testCase: %d %v", i, testCase,
		)
	}
}
