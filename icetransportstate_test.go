package webrtc

import (
	"testing"

	"github.com/pions/ice"
	"github.com/stretchr/testify/assert"
)

func TestICETransportState_String(t *testing.T) {
	testCases := []struct {
		state          ICETransportState
		expectedString string
	}{
		{ICETransportState(Unknown), unknownStr},
		{ICETransportStateNew, "new"},
		{ICETransportStateChecking, "checking"},
		{ICETransportStateConnected, "connected"},
		{ICETransportStateCompleted, "completed"},
		{ICETransportStateFailed, "failed"},
		{ICETransportStateDisconnected, "disconnected"},
		{ICETransportStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestICETransportState_Convert(t *testing.T) {
	testCases := []struct {
		native ICETransportState
		ice    ice.ConnectionState
	}{
		{ICETransportState(Unknown), ice.ConnectionState(Unknown)},
		{ICETransportStateNew, ice.ConnectionStateNew},
		{ICETransportStateChecking, ice.ConnectionStateChecking},
		{ICETransportStateConnected, ice.ConnectionStateConnected},
		{ICETransportStateCompleted, ice.ConnectionStateCompleted},
		{ICETransportStateFailed, ice.ConnectionStateFailed},
		{ICETransportStateDisconnected, ice.ConnectionStateDisconnected},
		{ICETransportStateClosed, ice.ConnectionStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.native.toICE(),
			testCase.ice,
			"testCase: %d %v", i, testCase,
		)
		assert.Equal(t,
			testCase.native,
			newICETransportStateFromICE(testCase.ice),
			"testCase: %d %v", i, testCase,
		)
	}
}
