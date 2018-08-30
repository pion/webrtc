package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCRtpTransceiverDirection(t *testing.T) {
	testCases := []struct {
		priorityString   string
		expectedPriority RTCRtpTransceiverDirection
	}{
		{"unknown", RTCRtpTransceiverDirection(Unknown)},
		{"sendrecv", RTCRtpTransceiverDirectionSendrecv},
		{"sendonly", RTCRtpTransceiverDirectionSendonly},
		{"recvonly", RTCRtpTransceiverDirectionRecvonly},
		{"inactive", RTCRtpTransceiverDirectionInactive},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			NewRTCRtpTransceiverDirection(testCase.priorityString),
			testCase.expectedPriority,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCRtpTransceiverDirection_String(t *testing.T) {
	testCases := []struct {
		priority       RTCRtpTransceiverDirection
		expectedString string
	}{
		{RTCRtpTransceiverDirection(Unknown), "unknown"},
		{RTCRtpTransceiverDirectionSendrecv, "sendrecv"},
		{RTCRtpTransceiverDirectionSendonly, "sendonly"},
		{RTCRtpTransceiverDirectionRecvonly, "recvonly"},
		{RTCRtpTransceiverDirectionInactive, "inactive"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.priority.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
