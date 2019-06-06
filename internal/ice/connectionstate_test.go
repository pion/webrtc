package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConnectionState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState ConnectionState
	}{
		{unknownStr, ConnectionState(Unknown)},
		{"new", ConnectionStateNew},
		{"checking", ConnectionStateChecking},
		{"connected", ConnectionStateConnected},
		{"completed", ConnectionStateCompleted},
		{"disconnected", ConnectionStateDisconnected},
		{"failed", ConnectionStateFailed},
		{"closed", ConnectionStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			NewConnectionState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestConnectionState_String(t *testing.T) {
	testCases := []struct {
		state          ConnectionState
		expectedString string
	}{
		{ConnectionState(Unknown), unknownStr},
		{ConnectionStateNew, "new"},
		{ConnectionStateChecking, "checking"},
		{ConnectionStateConnected, "connected"},
		{ConnectionStateCompleted, "completed"},
		{ConnectionStateDisconnected, "disconnected"},
		{ConnectionStateFailed, "failed"},
		{ConnectionStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
