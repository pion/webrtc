package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCSignalingState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState RTCSignalingState
	}{
		{"unknown", RTCSignalingState(Unknown)},
		{"stable", RTCSignalingStateStable},
		{"have-local-offer", RTCSignalingStateHaveLocalOffer},
		{"have-remote-offer", RTCSignalingStateHaveRemoteOffer},
		{"have-local-pranswer", RTCSignalingStateHaveLocalPranswer},
		{"have-remote-pranswer", RTCSignalingStateHaveRemotePranswer},
		{"closed", RTCSignalingStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			newRTCSignalingState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCSignalingState_String(t *testing.T) {
	testCases := []struct {
		state          RTCSignalingState
		expectedString string
	}{
		{RTCSignalingState(Unknown), "unknown"},
		{RTCSignalingStateStable, "stable"},
		{RTCSignalingStateHaveLocalOffer, "have-local-offer"},
		{RTCSignalingStateHaveRemoteOffer, "have-remote-offer"},
		{RTCSignalingStateHaveLocalPranswer, "have-local-pranswer"},
		{RTCSignalingStateHaveRemotePranswer, "have-remote-pranswer"},
		{RTCSignalingStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
