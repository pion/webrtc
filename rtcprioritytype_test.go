package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCPriorityType(t *testing.T) {
	testCases := []struct {
		priorityString   string
		priorityUint16   uint16
		expectedPriority RTCPriorityType
	}{
		{"unknown", 0, RTCPriorityType(Unknown)},
		{"very-low", 100, RTCPriorityTypeVeryLow},
		{"low", 200, RTCPriorityTypeLow},
		{"medium", 300, RTCPriorityTypeMedium},
		{"high", 1000, RTCPriorityTypeHigh},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedPriority,
			newRTCPriorityTypeFromString(testCase.priorityString),
			"testCase: %d %v", i, testCase,
		)

		// There is no uint that produces generate RTCPriorityType(Unknown).
		if i == 0 {
			continue
		}

		assert.Equal(t,
			testCase.expectedPriority,
			newRTCPriorityTypeFromUint16(testCase.priorityUint16),
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
			testCase.expectedString,
			testCase.priority.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
