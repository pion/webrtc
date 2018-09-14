package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCIceCredentialType(t *testing.T) {
	testCases := []struct {
		credentialTypeString   string
		expectedCredentialType RTCIceCredentialType
	}{
		{"unknown", RTCIceCredentialType(Unknown)},
		{"password", RTCIceCredentialTypePassword},
		{"oauth", RTCIceCredentialTypeOauth},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedCredentialType,
			newRTCIceCredentialType(testCase.credentialTypeString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCIceCredentialType_String(t *testing.T) {
	testCases := []struct {
		credentialType RTCIceCredentialType
		expectedString string
	}{
		{RTCIceCredentialType(Unknown), "unknown"},
		{RTCIceCredentialTypePassword, "password"},
		{RTCIceCredentialTypeOauth, "oauth"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.credentialType.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
