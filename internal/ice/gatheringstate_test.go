package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGatheringState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState GatheringState
	}{
		{unknownStr, GatheringState(Unknown)},
		{"new", GatheringStateNew},
		{"gathering", GatheringStateGathering},
		{"complete", GatheringStateComplete},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			NewGatheringState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestGatheringState_String(t *testing.T) {
	testCases := []struct {
		state          GatheringState
		expectedString string
	}{
		{GatheringState(Unknown), unknownStr},
		{GatheringStateNew, "new"},
		{GatheringStateGathering, "gathering"},
		{GatheringStateComplete, "complete"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
