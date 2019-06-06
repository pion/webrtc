package ice

import (
	"testing"

	"github.com/pion/ice"
	"github.com/stretchr/testify/assert"
)

func TestTransportState_String(t *testing.T) {
	testCases := []struct {
		state          TransportState
		expectedString string
	}{
		{TransportState(Unknown), unknownStr},
		{TransportStateNew, "new"},
		{TransportStateChecking, "checking"},
		{TransportStateConnected, "connected"},
		{TransportStateCompleted, "completed"},
		{TransportStateFailed, "failed"},
		{TransportStateDisconnected, "disconnected"},
		{TransportStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestTransportState_Convert(t *testing.T) {
	testCases := []struct {
		native TransportState
		ice    ice.ConnectionState
	}{
		{TransportState(Unknown), ice.ConnectionState(Unknown)},
		{TransportStateNew, ice.ConnectionStateNew},
		{TransportStateChecking, ice.ConnectionStateChecking},
		{TransportStateConnected, ice.ConnectionStateConnected},
		{TransportStateCompleted, ice.ConnectionStateCompleted},
		{TransportStateFailed, ice.ConnectionStateFailed},
		{TransportStateDisconnected, ice.ConnectionStateDisconnected},
		{TransportStateClosed, ice.ConnectionStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.native.toICE(),
			testCase.ice,
			"testCase: %d %v", i, testCase,
		)
		assert.Equal(t,
			testCase.native,
			newTransportStateFromICE(testCase.ice),
			"testCase: %d %v", i, testCase,
		)
	}
}
