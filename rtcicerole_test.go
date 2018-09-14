package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCIceRole(t *testing.T) {
	testCases := []struct {
		roleString   string
		expectedRole RTCIceRole
	}{
		{"unknown", RTCIceRole(Unknown)},
		{"controlling", RTCIceRoleControlling},
		{"controlled", RTCIceRoleControlled},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedRole,
			newRTCIceRole(testCase.roleString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCIceRole_String(t *testing.T) {
	testCases := []struct {
		proto          RTCIceRole
		expectedString string
	}{
		{RTCIceRole(Unknown), "unknown"},
		{RTCIceRoleControlling, "controlling"},
		{RTCIceRoleControlled, "controlled"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.proto.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
