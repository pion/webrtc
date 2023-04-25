// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDataChannelState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState DataChannelState
	}{
		{unknownStr, DataChannelState(Unknown)},
		{"connecting", DataChannelStateConnecting},
		{"open", DataChannelStateOpen},
		{"closing", DataChannelStateClosing},
		{"closed", DataChannelStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			newDataChannelState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestDataChannelState_String(t *testing.T) {
	testCases := []struct {
		state          DataChannelState
		expectedString string
	}{
		{DataChannelState(Unknown), unknownStr},
		{DataChannelStateConnecting, "connecting"},
		{DataChannelStateOpen, "open"},
		{DataChannelStateClosing, "closing"},
		{DataChannelStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
