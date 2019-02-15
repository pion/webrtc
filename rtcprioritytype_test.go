package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPriorityType(t *testing.T) {
	testCases := []struct {
		priorityString   string
		priorityUint16   uint16
		expectedPriority PriorityType
	}{
		{unknownStr, 0, PriorityType(Unknown)},
		{"very-low", 100, PriorityTypeVeryLow},
		{"low", 200, PriorityTypeLow},
		{"medium", 300, PriorityTypeMedium},
		{"high", 1000, PriorityTypeHigh},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedPriority,
			newPriorityTypeFromString(testCase.priorityString),
			"testCase: %d %v", i, testCase,
		)

		// There is no uint that produces generate PriorityType(Unknown).
		if i == 0 {
			continue
		}

		assert.Equal(t,
			testCase.expectedPriority,
			newPriorityTypeFromUint16(testCase.priorityUint16),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestPriorityType_String(t *testing.T) {
	testCases := []struct {
		priority       PriorityType
		expectedString string
	}{
		{PriorityType(Unknown), unknownStr},
		{PriorityTypeVeryLow, "very-low"},
		{PriorityTypeLow, "low"},
		{PriorityTypeMedium, "medium"},
		{PriorityTypeHigh, "high"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.priority.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
