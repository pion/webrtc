package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestICEGathererState_String(t *testing.T) {
	testCases := []struct {
		state          ICEGathererState
		expectedString string
	}{
		{ICEGathererState(Unknown), unknownStr},
		{ICEGathererStateNew, "new"},
		{ICEGathererStateGathering, "gathering"},
		{ICEGathererStateComplete, "complete"},
		{ICEGathererStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
