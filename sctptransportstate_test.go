// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSCTPTransportState(t *testing.T) {
	testCases := []struct {
		transportStateString   string
		expectedTransportState SCTPTransportState
	}{
		{unknownStr, SCTPTransportState(Unknown)},
		{"connecting", SCTPTransportStateConnecting},
		{"connected", SCTPTransportStateConnected},
		{"closed", SCTPTransportStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedTransportState,
			newSCTPTransportState(testCase.transportStateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestSCTPTransportState_String(t *testing.T) {
	testCases := []struct {
		transportState SCTPTransportState
		expectedString string
	}{
		{SCTPTransportState(Unknown), unknownStr},
		{SCTPTransportStateConnecting, "connecting"},
		{SCTPTransportStateConnected, "connected"},
		{SCTPTransportStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.transportState.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
