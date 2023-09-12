// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDTLSTransportState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState DTLSTransportState
	}{
		{ErrUnknownType.Error(), DTLSTransportStateUnknown},
		{"new", DTLSTransportStateNew},
		{"connecting", DTLSTransportStateConnecting},
		{"connected", DTLSTransportStateConnected},
		{"closed", DTLSTransportStateClosed},
		{"failed", DTLSTransportStateFailed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			newDTLSTransportState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestDTLSTransportState_String(t *testing.T) {
	testCases := []struct {
		state          DTLSTransportState
		expectedString string
	}{
		{DTLSTransportStateUnknown, ErrUnknownType.Error()},
		{DTLSTransportStateNew, "new"},
		{DTLSTransportStateConnecting, "connecting"},
		{DTLSTransportStateConnected, "connected"},
		{DTLSTransportStateClosed, "closed"},
		{DTLSTransportStateFailed, "failed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
