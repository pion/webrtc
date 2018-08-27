package webrtc

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
			NewRTCSignalingState(testCase.stateString),
			testCase.expectedState,
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
			testCase.state.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
