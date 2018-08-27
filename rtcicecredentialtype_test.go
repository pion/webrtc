package webrtc

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
			NewRTCIceCredentialType(testCase.credentialTypeString),
			testCase.expectedCredentialType,
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
			testCase.credentialType.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
