package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRTCQuicRole_String(t *testing.T) {
	testCases := []struct {
		role           RTCQuicRole
		expectedString string
	}{
		{RTCQuicRole(Unknown), unknownStr},
		{RTCQuicRoleAuto, "auto"},
		{RTCQuicRoleClient, "client"},
		{RTCQuicRoleServer, "server"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.role.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
