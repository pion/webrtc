// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestICECandidateType(t *testing.T) {
	testCases := []struct {
		typeString   string
		shouldFail   bool
		expectedType ICECandidateType
	}{
		{ErrUnknownType.Error(), true, ICECandidateTypeUnknown},
		{"host", false, ICECandidateTypeHost},
		{"srflx", false, ICECandidateTypeSrflx},
		{"prflx", false, ICECandidateTypePrflx},
		{"relay", false, ICECandidateTypeRelay},
	}

	for i, testCase := range testCases {
		actual, err := NewICECandidateType(testCase.typeString)
		if testCase.shouldFail {
			assert.Error(t, err, "testCase: %d %v", i, testCase)
		} else {
			assert.NoError(t, err, "testCase: %d %v", i, testCase)
		}
		assert.Equal(t,
			testCase.expectedType,
			actual,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestICECandidateType_String(t *testing.T) {
	testCases := []struct {
		cType          ICECandidateType
		expectedString string
	}{
		{ICECandidateTypeUnknown, ErrUnknownType.Error()},
		{ICECandidateTypeHost, "host"},
		{ICECandidateTypeSrflx, "srflx"},
		{ICECandidateTypePrflx, "prflx"},
		{ICECandidateTypeRelay, "relay"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.cType.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
