package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRTCDtlsRole_String(t *testing.T) {
	testCases := []struct {
		role           RTCDtlsRole
		expectedString string
	}{
		{RTCDtlsRole(Unknown), unknownStr},
		{RTCDtlsRoleAuto, "auto"},
		{RTCDtlsRoleClient, "client"},
		{RTCDtlsRoleServer, "server"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.role.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
