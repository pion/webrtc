// +build !js

package ice

import (
	"testing"

	"github.com/pion/ice"
	"github.com/pion/webrtc/v2/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
)

func TestServer_validate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		testCases := []struct {
			server           Server
			expectedValidate bool
		}{
			{Server{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     "placeholder",
				CredentialType: CredentialTypePassword,
			}, true},
			{Server{
				URLs:     []string{"turn:192.158.29.39?transport=udp"},
				Username: "unittest",
				Credential: OAuthCredential{
					MACKey:      "WmtzanB3ZW9peFhtdm42NzUzNG0=",
					AccessToken: "AAwg3kPHWPfvk9bDFL936wYvkoctMADzQ5VhNDgeMR3+ZlZ35byg972fW8QjpEl7bx91YLBPFsIhsxloWcXPhA==",
				},
				CredentialType: CredentialTypeOauth,
			}, true},
		}

		for i, testCase := range testCases {
			_, err := testCase.server.urls()
			assert.Nil(t, err, "testCase: %d %v", i, testCase)
		}
	})
	t.Run("Failure", func(t *testing.T) {
		testCases := []struct {
			server      Server
			expectedErr error
		}{
			{Server{
				URLs: []string{"turn:192.158.29.39?transport=udp"},
			}, &rtcerr.InvalidAccessError{Err: ErrNoTurnCredencials}},
			{Server{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: CredentialTypePassword,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}},
			{Server{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: CredentialTypeOauth,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}},
			{Server{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: Unknown,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}},
			{Server{
				URLs:           []string{"stun:google.de?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: CredentialTypeOauth,
			}, ice.ErrSTUNQuery},
		}

		for i, testCase := range testCases {
			_, err := testCase.server.urls()
			assert.EqualError(t,
				err,
				testCase.expectedErr.Error(),
				"testCase: %d %v", i, testCase,
			)
		}
	})
}
