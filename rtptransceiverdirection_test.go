package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTPTransceiverDirection(t *testing.T) {
	testCases := []struct {
		priorityString   string
		expectedPriority RTPTransceiverDirection
	}{
		{unknownStr, RTPTransceiverDirection(Unknown)},
		{"sendrecv", RTPTransceiverDirectionSendrecv},
		{"sendonly", RTPTransceiverDirectionSendonly},
		{"recvonly", RTPTransceiverDirectionRecvonly},
		{"inactive", RTPTransceiverDirectionInactive},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			NewRTPTransceiverDirection(testCase.priorityString),
			testCase.expectedPriority,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTPTransceiverDirection_String(t *testing.T) {
	testCases := []struct {
		priority       RTPTransceiverDirection
		expectedString string
	}{
		{RTPTransceiverDirection(Unknown), unknownStr},
		{RTPTransceiverDirectionSendrecv, "sendrecv"},
		{RTPTransceiverDirectionSendonly, "sendonly"},
		{RTPTransceiverDirectionRecvonly, "recvonly"},
		{RTPTransceiverDirectionInactive, "inactive"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.priority.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
