package webrtc

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
			NewRTCIceConnectionState(testCase.stateString),
			testCase.expectedState,
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
			testCase.state.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
