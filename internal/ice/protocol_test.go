package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProtocol(t *testing.T) {
	testCases := []struct {
		protoString   string
		shouldFail    bool
		expectedProto Protocol
	}{
		{unknownStr, true, Protocol(Unknown)},
		{"udp", false, ProtocolUDP},
		{"tcp", false, ProtocolTCP},
		{"UDP", false, ProtocolUDP},
		{"TCP", false, ProtocolTCP},
	}

	for i, testCase := range testCases {
		actual, err := NewProtocol(testCase.protoString)
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

func TestProtocol_String(t *testing.T) {
	testCases := []struct {
		proto          Protocol
		expectedString string
	}{
		{Protocol(Unknown), unknownStr},
		{ProtocolUDP, "udp"},
		{ProtocolTCP, "tcp"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.proto.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
