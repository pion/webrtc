package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCDataChannelState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState RTCDataChannelState
	}{
		{"unknown", RTCDataChannelState(Unknown)},
		{"connecting", RTCDataChannelStateConnecting},
		{"open", RTCDataChannelStateOpen},
		{"closing", RTCDataChannelStateClosing},
		{"closed", RTCDataChannelStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			newRTCDataChannelState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCDataChannelState_String(t *testing.T) {
	testCases := []struct {
		state          RTCDataChannelState
		expectedString string
	}{
		{RTCDataChannelState(Unknown), "unknown"},
		{RTCDataChannelStateConnecting, "connecting"},
		{RTCDataChannelStateOpen, "open"},
		{RTCDataChannelStateClosing, "closing"},
		{RTCDataChannelStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
