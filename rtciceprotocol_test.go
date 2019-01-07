package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCIceProtocol(t *testing.T) {
	testCases := []struct {
		protoString   string
		shouldFail    bool
		expectedProto RTCIceProtocol
	}{
		{unknownStr, true, RTCIceProtocol(Unknown)},
		{"udp", false, RTCIceProtocolUDP},
		{"tcp", false, RTCIceProtocolTCP},
	}

	for i, testCase := range testCases {
		actual, err := newRTCIceProtocol(testCase.protoString)
		if (err != nil) != testCase.shouldFail {
			t.Error(err)
		}
		assert.Equal(t,
			testCase.expectedProto,
			actual,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCIceProtocol_String(t *testing.T) {
	testCases := []struct {
		proto          RTCIceProtocol
		expectedString string
	}{
		{RTCIceProtocol(Unknown), unknownStr},
		{RTCIceProtocolUDP, "udp"},
		{RTCIceProtocolTCP, "tcp"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.proto.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
