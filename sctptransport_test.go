// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import "testing"

func TestGenerateDataChannelID(t *testing.T) {
	sctpTransportWithChannels := func(ids []uint16) *SCTPTransport {
		ret := &SCTPTransport{
			dataChannels:       []*DataChannel{},
			dataChannelIDsUsed: make(map[uint16]struct{}),
		}

		for i := range ids {
			id := ids[i]
			ret.dataChannels = append(ret.dataChannels, &DataChannel{id: &id})
			ret.dataChannelIDsUsed[id] = struct{}{}
		}

		return ret
	}

	testCases := []struct {
		role   DTLSRole
		s      *SCTPTransport
		result uint16
	}{
		{DTLSRoleClient, sctpTransportWithChannels([]uint16{}), 0},
		{DTLSRoleClient, sctpTransportWithChannels([]uint16{1}), 0},
		{DTLSRoleClient, sctpTransportWithChannels([]uint16{0}), 2},
		{DTLSRoleClient, sctpTransportWithChannels([]uint16{0, 2}), 4},
		{DTLSRoleClient, sctpTransportWithChannels([]uint16{0, 4}), 2},
		{DTLSRoleServer, sctpTransportWithChannels([]uint16{}), 1},
		{DTLSRoleServer, sctpTransportWithChannels([]uint16{0}), 1},
		{DTLSRoleServer, sctpTransportWithChannels([]uint16{1}), 3},
		{DTLSRoleServer, sctpTransportWithChannels([]uint16{1, 3}), 5},
		{DTLSRoleServer, sctpTransportWithChannels([]uint16{1, 5}), 3},
	}
	for _, testCase := range testCases {
		idPtr := new(uint16)
		err := testCase.s.generateAndSetDataChannelID(testCase.role, &idPtr)
		if err != nil {
			t.Errorf("failed to generate id: %v", err)
			return
		}
		if *idPtr != testCase.result {
			t.Errorf("Wrong id: %d expected %d", *idPtr, testCase.result)
		}
		if _, ok := testCase.s.dataChannelIDsUsed[*idPtr]; !ok {
			t.Errorf("expected new id to be added to the map: %d", *idPtr)
		}
	}
}
