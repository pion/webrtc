// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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

func TestSCTPTransportOnClose(t *testing.T) {
	offerPC, answerPC, err := newPair()
	require.NoError(t, err)

	defer closePairNow(t, offerPC, answerPC)

	answerPC.OnDataChannel(func(dc *DataChannel) {
		dc.OnMessage(func(_ DataChannelMessage) {
			if err1 := dc.Send([]byte("hello")); err1 != nil {
				t.Error("failed to send message")
			}
		})
	})

	recvMsg := make(chan struct{}, 1)
	offerPC.OnConnectionStateChange(func(state PeerConnectionState) {
		if state == PeerConnectionStateConnected {
			defer func() {
				offerPC.OnConnectionStateChange(nil)
			}()

			dc, createErr := offerPC.CreateDataChannel(expectedLabel, nil)
			if createErr != nil {
				t.Errorf("Failed to create a PC pair for testing")
				return
			}
			dc.OnMessage(func(msg DataChannelMessage) {
				if !bytes.Equal(msg.Data, []byte("hello")) {
					t.Error("invalid msg received")
				}
				recvMsg <- struct{}{}
			})
			dc.OnOpen(func() {
				if err1 := dc.Send([]byte("hello")); err1 != nil {
					t.Error("failed to send initial msg", err1)
				}
			})
		}
	})

	err = signalPair(offerPC, answerPC)
	require.NoError(t, err)

	select {
	case <-recvMsg:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	// setup SCTP OnClose callback
	ch := make(chan error, 1)
	answerPC.SCTP().OnClose(func(err error) {
		ch <- err
	})

	err = offerPC.Close() // This will trigger sctp onclose callback on remote
	require.NoError(t, err)

	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
}
