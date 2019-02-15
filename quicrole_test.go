package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQUICRole_String(t *testing.T) {
	testCases := []struct {
		role           QUICRole
		expectedString string
	}{
		{QUICRole(Unknown), unknownStr},
		{QUICRoleAuto, "auto"},
		{QUICRoleClient, "client"},
		{QUICRoleServer, "server"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.role.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
