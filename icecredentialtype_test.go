package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewICECredentialType(t *testing.T) {
	testCases := []struct {
		credentialTypeString   string
		expectedCredentialType ICECredentialType
	}{
		{"password", ICECredentialTypePassword},
		{"oauth", ICECredentialTypeOauth},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedCredentialType,
			newICECredentialType(testCase.credentialTypeString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestICECredentialType_String(t *testing.T) {
	testCases := []struct {
		credentialType ICECredentialType
		expectedString string
	}{
		{ICECredentialTypePassword, "password"},
		{ICECredentialTypeOauth, "oauth"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.credentialType.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
