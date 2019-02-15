package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewICEConnectionState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState ICEConnectionState
	}{
		{unknownStr, ICEConnectionState(Unknown)},
		{"new", ICEConnectionStateNew},
		{"checking", ICEConnectionStateChecking},
		{"connected", ICEConnectionStateConnected},
		{"completed", ICEConnectionStateCompleted},
		{"disconnected", ICEConnectionStateDisconnected},
		{"failed", ICEConnectionStateFailed},
		{"closed", ICEConnectionStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			newICEConnectionState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestICEConnectionState_String(t *testing.T) {
	testCases := []struct {
		state          ICEConnectionState
		expectedString string
	}{
		{ICEConnectionState(Unknown), unknownStr},
		{ICEConnectionStateNew, "new"},
		{ICEConnectionStateChecking, "checking"},
		{ICEConnectionStateConnected, "connected"},
		{ICEConnectionStateCompleted, "completed"},
		{ICEConnectionStateDisconnected, "disconnected"},
		{ICEConnectionStateFailed, "failed"},
		{ICEConnectionStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
