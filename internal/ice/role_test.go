package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRole(t *testing.T) {
	testCases := []struct {
		roleString   string
		expectedRole Role
	}{
		{unknownStr, Role(Unknown)},
		{"controlling", RoleControlling},
		{"controlled", RoleControlled},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedRole,
			newRole(testCase.roleString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRole_String(t *testing.T) {
	testCases := []struct {
		proto          Role
		expectedString string
	}{
		{Role(Unknown), unknownStr},
		{RoleControlling, "controlling"},
		{RoleControlled, "controlled"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.proto.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
