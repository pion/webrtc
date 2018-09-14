package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCIceGatheringState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState RTCIceGatheringState
	}{
		{"unknown", RTCIceGatheringState(Unknown)},
		{"new", RTCIceGatheringStateNew},
		{"gathering", RTCIceGatheringStateGathering},
		{"complete", RTCIceGatheringStateComplete},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			newRTCIceGatheringState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCIceGatheringState_String(t *testing.T) {
	testCases := []struct {
		state          RTCIceGatheringState
		expectedString string
	}{
		{RTCIceGatheringState(Unknown), "unknown"},
		{RTCIceGatheringStateNew, "new"},
		{RTCIceGatheringStateGathering, "gathering"},
		{RTCIceGatheringStateComplete, "complete"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
