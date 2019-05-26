package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComponent(t *testing.T) {
	testCases := []struct {
		componentString   string
		expectedComponent Component
	}{
		{unknownStr, Component(Unknown)},
		{"rtp", ComponentRTP},
		{"rtcp", ComponentRTCP},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			newComponent(testCase.componentString),
			testCase.expectedComponent,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestComponent_String(t *testing.T) {
	testCases := []struct {
		state          Component
		expectedString string
	}{
		{Component(Unknown), unknownStr},
		{ComponentRTP, "rtp"},
		{ComponentRTCP, "rtcp"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.state.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
