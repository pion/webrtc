package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCIceConnectionState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState RTCIceConnectionState
	}{
		{"unknown", RTCIceConnectionState(Unknown)},
		{"new", RTCIceConnectionStateNew},
		{"checking", RTCIceConnectionStateChecking},
		{"connected", RTCIceConnectionStateConnected},
		{"completed", RTCIceConnectionStateCompleted},
		{"disconnected", RTCIceConnectionStateDisconnected},
		{"failed", RTCIceConnectionStateFailed},
		{"closed", RTCIceConnectionStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			newRTCIceConnectionState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCIceConnectionState_String(t *testing.T) {
	testCases := []struct {
		state          RTCIceConnectionState
		expectedString string
	}{
		{RTCIceConnectionState(Unknown), "unknown"},
		{RTCIceConnectionStateNew, "new"},
		{RTCIceConnectionStateChecking, "checking"},
		{RTCIceConnectionStateConnected, "connected"},
		{RTCIceConnectionStateCompleted, "completed"},
		{RTCIceConnectionStateDisconnected, "disconnected"},
		{RTCIceConnectionStateFailed, "failed"},
		{RTCIceConnectionStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
