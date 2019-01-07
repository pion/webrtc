package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRTCIceGathererState_String(t *testing.T) {
	testCases := []struct {
		state          RTCIceGathererState
		expectedString string
	}{
		{RTCIceGathererState(Unknown), unknownStr},
		{RTCIceGathererStateNew, "new"},
		{RTCIceGathererStateGathering, "gathering"},
		{RTCIceGathererStateComplete, "complete"},
		{RTCIceGathererStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
