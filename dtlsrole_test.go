// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"fmt"
	"testing"

	"github.com/pion/sdp/v3"
	"github.com/stretchr/testify/assert"
)

func TestDTLSRole_String(t *testing.T) {
	testCases := []struct {
		role           DTLSRole
		expectedString string
	}{
		{DTLSRole(Unknown), unknownStr},
		{DTLSRoleAuto, "auto"},
		{DTLSRoleClient, "client"},
		{DTLSRoleServer, "server"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.role.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestDTLSRoleFromRemoteSDP(t *testing.T) {
	parseSDP := func(raw string) *sdp.SessionDescription {
		parsed := &sdp.SessionDescription{}
		if err := parsed.Unmarshal([]byte(raw)); err != nil {
			panic(err)
		}
		return parsed
	}

	const noMedia = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
`

	const mediaNoSetup = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=application 47299 DTLS/SCTP 5000
c=IN IP4 192.168.20.129
`

	const mediaSetupDeclared = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=application 47299 DTLS/SCTP 5000
c=IN IP4 192.168.20.129
a=setup:%s
`

	testCases := []struct {
		test               string
		sessionDescription *sdp.SessionDescription
		expectedRole       DTLSRole
	}{
		{"nil SessionDescription", nil, DTLSRoleAuto},
		{"No MediaDescriptions", parseSDP(noMedia), DTLSRoleAuto},
		{"MediaDescription, no setup", parseSDP(mediaNoSetup), DTLSRoleAuto},
		{"MediaDescription, setup:actpass", parseSDP(fmt.Sprintf(mediaSetupDeclared, "actpass")), DTLSRoleAuto},
		{"MediaDescription, setup:passive", parseSDP(fmt.Sprintf(mediaSetupDeclared, "passive")), DTLSRoleServer},
		{"MediaDescription, setup:active", parseSDP(fmt.Sprintf(mediaSetupDeclared, "active")), DTLSRoleClient},
	}
	for _, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedRole,
			dtlsRoleFromRemoteSDP(testCase.sessionDescription),
			"TestDTLSRoleFromSDP (%s)", testCase.test,
		)
	}
}
