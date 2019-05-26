package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCredentialType(t *testing.T) {
	testCases := []struct {
		credentialTypeString   string
		expectedCredentialType CredentialType
	}{
		{"password", CredentialTypePassword},
		{"oauth", CredentialTypeOauth},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedCredentialType,
			newCredentialType(testCase.credentialTypeString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestCredentialType_String(t *testing.T) {
	testCases := []struct {
		credentialType CredentialType
		expectedString string
	}{
		{CredentialTypePassword, "password"},
		{CredentialTypeOauth, "oauth"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.credentialType.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
