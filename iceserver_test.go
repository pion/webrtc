// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"encoding/json"
	"testing"

	"github.com/pion/stun"
	"github.com/pion/webrtc/v3/pkg/rtcerr"
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
				URLs:           []string{"turn:[2001:db8:1234:5678::1]?transport=udp"},
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
			var iceServer ICEServer
			jsonobj, err := json.Marshal(testCase.iceServer)
			assert.NoError(t, err)
			err = json.Unmarshal(jsonobj, &iceServer)
			assert.NoError(t, err)
			assert.Equal(t, iceServer, testCase.iceServer)
			_, err = testCase.iceServer.urls()
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
			}, &rtcerr.InvalidAccessError{Err: ErrNoTurnCredentials}},
			{ICEServer{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: ICECredentialTypePassword,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredentials}},
			{ICEServer{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: ICECredentialTypeOauth,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredentials}},
			{ICEServer{
				URLs:           []string{"turn:192.158.29.39?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: Unknown,
			}, &rtcerr.InvalidAccessError{Err: ErrTurnCredentials}},
			{ICEServer{
				URLs:           []string{"stun:google.de?transport=udp"},
				Username:       "unittest",
				Credential:     false,
				CredentialType: ICECredentialTypeOauth,
			}, &rtcerr.InvalidAccessError{Err: stun.ErrSTUNQuery}},
		}

		for i, testCase := range testCases {
			_, err := testCase.iceServer.urls()
			assert.EqualError(t,
				err,
				testCase.expectedErr.Error(),
				"testCase: %d %v", i, testCase,
			)
		}
	})
	t.Run("JsonFailure", func(t *testing.T) {
		testCases := [][]byte{
			[]byte(`{"urls":"NOTAURL","username":"unittest","credential":"placeholder","credentialType":"password"}`),
			[]byte(`{"urls":["turn:[2001:db8:1234:5678::1]?transport=udp"],"username":"unittest","credential":"placeholder","credentialType":"invalid"}`),
			[]byte(`{"urls":["turn:[2001:db8:1234:5678::1]?transport=udp"],"username":6,"credential":"placeholder","credentialType":"password"}`),
			[]byte(`{"urls":["turn:192.158.29.39?transport=udp"],"username":"unittest","credential":{"Bad Object": true},"credentialType":"oauth"}`),
			[]byte(`{"urls":["turn:192.158.29.39?transport=udp"],"username":"unittest","credential":{"MACKey":"WmtzanB3ZW9peFhtdm42NzUzNG0=","AccessToken":null,"credentialType":"oauth"}`),
			[]byte(`{"urls":["turn:192.158.29.39?transport=udp"],"username":"unittest","credential":{"MACKey":"WmtzanB3ZW9peFhtdm42NzUzNG0=","AccessToken":null,"credentialType":"password"}`),
			[]byte(`{"urls":["turn:192.158.29.39?transport=udp"],"username":"unittest","credential":{"MACKey":1337,"AccessToken":"AAwg3kPHWPfvk9bDFL936wYvkoctMADzQ5VhNDgeMR3+ZlZ35byg972fW8QjpEl7bx91YLBPFsIhsxloWcXPhA=="},"credentialType":"oauth"}`),
		}
		for i, testCase := range testCases {
			var tc ICEServer
			err := json.Unmarshal(testCase, &tc)
			assert.Error(t, err, "testCase: %d %v", i, string(testCase))
		}
	})
}

func TestICEServerZeroValue(t *testing.T) {
	server := ICEServer{
		URLs:       []string{"turn:galene.org:1195"},
		Username:   "galene",
		Credential: "secret",
	}
	assert.Equal(t, server.CredentialType, ICECredentialTypePassword)
}
