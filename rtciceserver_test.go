package webrtc

import (
	"testing"

	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
)

func TestRTCIceServer_validate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		testCases := []struct {
			iceServer        RTCIceServer
			expectedValidate bool
		}{
			{RTCIceServer{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     "placeholder",
				CredentialType: RTCIceCredentialTypePassword,
			}, true},
			{RTCIceServer{
				URLs:     []string{"turn:192.158.29.39?transport=udp"},
				Username: "unittest",
				Credential: RTCOAuthCredential{
					MacKey:      "WmtzanB3ZW9peFhtdm42NzUzNG0=",
					AccessToken: "AAwg3kPHWPfvk9bDFL936wYvkoctMADzQ5VhNDgeMR3+ZlZ35byg972fW8QjpEl7bx91YLBPFsIhsxloWcXPhA==",
				},
				CredentialType: RTCIceCredentialTypeOauth,
			}, true},
		}

		for i, testCase := range testCases {
			assert.Nil(t, testCase.iceServer.validate(), "testCase: %d %v", i, testCase)
		}
	})
	t.Run("Failure", func(t *testing.T) {
		testCases := []struct {
			iceServer   RTCIceServer
			expectedErr error
		}{
			{RTCIceServer{
				URLs: []string{"turn:192.158.29.39?transport=udp"},
			}, &rtcerr.InvalidAccessError{Err: ErrNoTurnCredencials}},
			{RTCIceServer{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: RTCIceCredentialTypePassword,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}},
			{RTCIceServer{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: RTCIceCredentialTypeOauth,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}},
			{RTCIceServer{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: Unknown,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}},
			{RTCIceServer{
				URLs:           []string{"stun:google.de?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: RTCIceCredentialTypeOauth,
			}, &rtcerr.SyntaxError{Err: ice.ErrSTUNQuery}},
		}

		for i, testCase := range testCases {
			assert.EqualError(t,
				testCase.iceServer.validate(),
				testCase.expectedErr.Error(),
				"testCase: %d %v", i, testCase,
			)
		}
	})
}
