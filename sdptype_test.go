// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSDPType(t *testing.T) {
	testCases := []struct {
		sdpTypeString   string
		expectedSDPType SDPType
	}{
		{ErrUnknownType.Error(), SDPTypeUnknown},
		{"offer", SDPTypeOffer},
		{"pranswer", SDPTypePranswer},
		{"answer", SDPTypeAnswer},
		{"rollback", SDPTypeRollback},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedSDPType,
			NewSDPType(testCase.sdpTypeString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestSDPType_String(t *testing.T) {
	testCases := []struct {
		sdpType        SDPType
		expectedString string
	}{
		{SDPTypeUnknown, ErrUnknownType.Error()},
		{SDPTypeOffer, "offer"},
		{SDPTypePranswer, "pranswer"},
		{SDPTypeAnswer, "answer"},
		{SDPTypeRollback, "rollback"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.sdpType.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
