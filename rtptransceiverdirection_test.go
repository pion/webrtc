// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTPTransceiverDirection(t *testing.T) {
	testCases := []struct {
		directionString   string
		expectedDirection RTPTransceiverDirection
	}{
		{unknownStr, RTPTransceiverDirection(Unknown)},
		{"sendrecv", RTPTransceiverDirectionSendrecv},
		{"sendonly", RTPTransceiverDirectionSendonly},
		{"recvonly", RTPTransceiverDirectionRecvonly},
		{"inactive", RTPTransceiverDirectionInactive},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			NewRTPTransceiverDirection(testCase.directionString),
			testCase.expectedDirection,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTPTransceiverDirection_String(t *testing.T) {
	testCases := []struct {
		direction      RTPTransceiverDirection
		expectedString string
	}{
		{RTPTransceiverDirection(Unknown), unknownStr},
		{RTPTransceiverDirectionSendrecv, "sendrecv"},
		{RTPTransceiverDirectionSendonly, "sendonly"},
		{RTPTransceiverDirectionRecvonly, "recvonly"},
		{RTPTransceiverDirectionInactive, "inactive"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.direction.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
