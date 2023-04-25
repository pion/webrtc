// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewICEGatheringState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState ICEGatheringState
	}{
		{unknownStr, ICEGatheringState(Unknown)},
		{"new", ICEGatheringStateNew},
		{"gathering", ICEGatheringStateGathering},
		{"complete", ICEGatheringStateComplete},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			NewICEGatheringState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestICEGatheringState_String(t *testing.T) {
	testCases := []struct {
		state          ICEGatheringState
		expectedString string
	}{
		{ICEGatheringState(Unknown), unknownStr},
		{ICEGatheringStateNew, "new"},
		{ICEGatheringStateGathering, "gathering"},
		{ICEGatheringStateComplete, "complete"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
