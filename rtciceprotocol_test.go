package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCIceProtocol(t *testing.T) {
	testCases := []struct {
		protoString   string
		expectedProto RTCIceProtocol
	}{
		{"unknown", RTCIceProtocol(Unknown)},
		{"udp", RTCIceProtocolUDP},
		{"tcp", RTCIceProtocolTCP},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedProto,
			newRTCIceProtocol(testCase.protoString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCIceProtocol_String(t *testing.T) {
	testCases := []struct {
		proto          RTCIceProtocol
		expectedString string
	}{
		{RTCIceProtocol(Unknown), "unknown"},
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
