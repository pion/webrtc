// +build !js

package webrtc

import (
	"testing"

	"github.com/pions/ice"
	"github.com/pions/webrtc/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
)

func TestICEServer_validate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		testCases := []struct {
			iceServer        ICEServer
			expectedValidate bool
		}{
			{ICEServer{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     "placeholder",
				CredentialType: ICECredentialTypePassword,
			}, true},
			{ICEServer{
				URLs:     []string{"turn:192.158.29.39?transport=udp"},
				Username: "unittest",
				Credential: OAuthCredential{
					MACKey:      "WmtzanB3ZW9peFhtdm42NzUzNG0=",
					AccessToken: "AAwg3kPHWPfvk9bDFL936wYvkoctMADzQ5VhNDgeMR3+ZlZ35byg972fW8QjpEl7bx91YLBPFsIhsxloWcXPhA==",
				},
				CredentialType: ICECredentialTypeOauth,
			}, true},
		}

		for i, testCase := range testCases {
			_, err := testCase.iceServer.validate()
			assert.Nil(t, err, "testCase: %d %v", i, testCase)
		}
	})
	t.Run("Failure", func(t *testing.T) {
		testCases := []struct {
			iceServer   ICEServer
			expectedErr error
		}{
			{ICEServer{
				URLs: []string{"turn:192.158.29.39?transport=udp"},
			}, &rtcerr.InvalidAccessError{Err: ErrNoTurnCredencials}},
			{ICEServer{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: ICECredentialTypePassword,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}},
			{ICEServer{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: ICECredentialTypeOauth,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}},
			{ICEServer{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: Unknown,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}},
			{ICEServer{
				URLs:           []string{"stun:google.de?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: ICECredentialTypeOauth,
			}, &rtcerr.SyntaxError{Err: ice.ErrSTUNQuery}},
		}

		for i, testCase := range testCases {
			_, err := testCase.iceServer.validate()
			assert.EqualError(t,
				err,
				testCase.expectedErr.Error(),
				"testCase: %d %v", i, testCase,
			)
		}
	})
}
