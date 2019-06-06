package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGathererState_String(t *testing.T) {
	testCases := []struct {
		state          GathererState
		expectedString string
	}{
		{GathererState(Unknown), unknownStr},
		{GathererStateNew, "new"},
		{GathererStateGathering, "gathering"},
		{GathererStateComplete, "complete"},
		{GathererStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
