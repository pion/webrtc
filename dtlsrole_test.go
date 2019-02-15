package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDTLSRole_String(t *testing.T) {
	testCases := []struct {
		role           DTLSRole
		expectedString string
	}{
		{DTLSRole(Unknown), unknownStr},
		{DTLSRoleAuto, "auto"},
		{DTLSRoleClient, "client"},
		{DTLSRoleServer, "server"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.role.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
