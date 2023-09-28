// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"encoding/json"
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
		tpe, err := newICECredentialType(testCase.credentialTypeString)
		assert.NoError(t, err)
		assert.Equal(t,
			testCase.expectedCredentialType, tpe,
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

func TestICECredentialType_new(t *testing.T) {
	testCases := []struct {
		credentialType ICECredentialType
		expectedString string
	}{
		{ICECredentialTypePassword, "password"},
		{ICECredentialTypeOauth, "oauth"},
	}

	for i, testCase := range testCases {
		tpe, err := newICECredentialType(testCase.expectedString)
		assert.NoError(t, err)
		assert.Equal(t,
			tpe, testCase.credentialType,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestICECredentialType_Json(t *testing.T) {
	testCases := []struct {
		credentialType     ICECredentialType
		jsonRepresentation []byte
	}{
		{ICECredentialTypePassword, []byte("\"password\"")},
		{ICECredentialTypeOauth, []byte("\"oauth\"")},
	}

	for i, testCase := range testCases {
		m, err := json.Marshal(testCase.credentialType)
		assert.NoError(t, err)
		assert.Equal(t,
			testCase.jsonRepresentation,
			m,
			"Marshal testCase: %d %v", i, testCase,
		)
		var ct ICECredentialType
		err = json.Unmarshal(testCase.jsonRepresentation, &ct)
		assert.NoError(t, err)
		assert.Equal(t,
			testCase.credentialType,
			ct,
			"Unmarshal testCase: %d %v", i, testCase,
		)
	}

	{
		ct := ICECredentialType(1000)
		err := json.Unmarshal([]byte("\"invalid\""), &ct)
		assert.Error(t, err)
		assert.Equal(t, ct, ICECredentialType(1000))
		err = json.Unmarshal([]byte("\"invalid"), &ct)
		assert.Error(t, err)
		assert.Equal(t, ct, ICECredentialType(1000))
	}
}
