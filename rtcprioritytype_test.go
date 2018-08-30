package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCPriorityType(t *testing.T) {
	testCases := []struct {
		priorityString   string
		expectedPriority RTCPriorityType
	}{
		{"unknown", RTCPriorityType(Unknown)},
		{"very-low", RTCPriorityTypeVeryLow},
		{"low", RTCPriorityTypeLow},
		{"medium", RTCPriorityTypeMedium},
		{"high", RTCPriorityTypeHigh},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			NewRTCPriorityType(testCase.priorityString),
			testCase.expectedPriority,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCPriorityType_String(t *testing.T) {
	testCases := []struct {
		priority       RTCPriorityType
		expectedString string
	}{
		{RTCPriorityType(Unknown), "unknown"},
		{RTCPriorityTypeVeryLow, "very-low"},
		{RTCPriorityTypeLow, "low"},
		{RTCPriorityTypeMedium, "medium"},
		{RTCPriorityTypeHigh, "high"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.priority.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
