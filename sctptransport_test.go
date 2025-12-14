// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"bufio"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
		assert.NoError(t, err, "failed to generate data channel id")
		assert.Equal(t, testCase.result, *idPtr)
		assert.Contains(
			t, testCase.s.dataChannelIDsUsed, *idPtr,
			"expected new id to be added to the map",
		)
	}
}

func TestSCTPTransportOnClose(t *testing.T) {
	offerPC, answerPC, err := newPair()
	require.NoError(t, err)

	defer closePairNow(t, offerPC, answerPC)

	answerPC.OnDataChannel(func(dc *DataChannel) {
		dc.OnMessage(func(_ DataChannelMessage) {
			assert.NoError(t, dc.Send([]byte("hello")), "failed to send message")
		})
	})

	recvMsg := make(chan struct{}, 1)
	offerPC.OnConnectionStateChange(func(state PeerConnectionState) {
		if state == PeerConnectionStateConnected {
			defer func() {
				offerPC.OnConnectionStateChange(nil)
			}()

			dc, createErr := offerPC.CreateDataChannel(expectedLabel, nil)
			assert.NoError(t, createErr, "Failed to create a PC pair for testing")
			dc.OnMessage(func(msg DataChannelMessage) {
				assert.Equal(
					t, []byte("hello"), msg.Data,
					"invalid msg received",
				)
				recvMsg <- struct{}{}
			})
			dc.OnOpen(func() {
				assert.NoError(t, dc.Send([]byte("hello")), "failed to send initial msg")
			})
		}
	})

	err = signalPair(offerPC, answerPC)
	require.NoError(t, err)

	select {
	case <-recvMsg:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "timed out")
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
	case <-time.After(15 * time.Second):
		assert.Fail(t, "timed out")
	}
}

// TestSCTPTransportOnCloseImmediate tests that OnClose fires immediately
// when Stop() is called directly on the SCTP transport, even if acceptDataChannels
// is blocked waiting for a new data channel. This test would fail "sometimes" without the fix
// because without the check before datachannel.Accept(), the goroutine would be
// blocked in Accept() and might not detect the closure until Accept() returns.
func TestSCTPTransportOnCloseImmediate(t *testing.T) {
	offerPC, answerPC, err := newPair()
	assert.NoError(t, err)

	defer closePairNow(t, offerPC, answerPC)

	connected := make(chan struct{}, 1)
	offerPC.OnConnectionStateChange(func(state PeerConnectionState) {
		if state == PeerConnectionStateConnected {
			connected <- struct{}{}
		}
	})

	err = signalPair(offerPC, answerPC)
	assert.NoError(t, err)

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "connection establishment timed out")

		return
	}

	// Create and open a data channel to ensure SCTP is fully established
	// and acceptDataChannels goroutine has processed it and is back in Accept()
	dc, err := offerPC.CreateDataChannel("test", nil)
	assert.NoError(t, err)

	dcOpened := make(chan struct{}, 1)
	dc.OnOpen(func() {
		dcOpened <- struct{}{}
	})

	select {
	case <-dcOpened:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "data channel open timed out")

		return
	}

	// wait a bit to ensure acceptDataChannels loop is back in Accept()
	// This increases the chance that Accept() is blocking when we call Stop()
	time.Sleep(10 * time.Millisecond)

	onCloseFired := make(chan error, 1)
	answerPC.SCTP().OnClose(func(err error) {
		onCloseFired <- err
	})

	err = answerPC.SCTP().Stop()
	assert.NoError(t, err)

	select {
	case <-onCloseFired:
	case <-time.After(50 * time.Millisecond):
		assert.Fail(t, "OnClose did not fire immediately")
	}
}

func TestSCTPTransportOutOfBandNegotiatedDataChannelDetach(t *testing.T) { //nolint:cyclop
	// nolint:varnamelen
	const N = 10
	done := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			// Use Detach data channels mode
			s := SettingEngine{}
			s.DetachDataChannels()
			api := NewAPI(WithSettingEngine(s))

			// Set up two peer connections.
			config := Configuration{}
			offerPC, err := api.NewPeerConnection(config)
			assert.NoError(t, err)
			answerPC, err := api.NewPeerConnection(config)
			assert.NoError(t, err)

			defer closePairNow(t, offerPC, answerPC)
			defer func() { done <- struct{}{} }()

			negotiated := true
			id := uint16(0)
			readDetach := make(chan struct{})
			dc1, err := offerPC.CreateDataChannel("", &DataChannelInit{
				Negotiated: &negotiated,
				ID:         &id,
			})
			assert.NoError(t, err)

			dc1.OnOpen(func() {
				_, _ = dc1.Detach()
				close(readDetach)
			})

			writeDetach := make(chan struct{})
			dc2, err := answerPC.CreateDataChannel("", &DataChannelInit{
				Negotiated: &negotiated,
				ID:         &id,
			})
			assert.NoError(t, err)

			dc2.OnOpen(func() {
				_, _ = dc2.Detach()
				close(writeDetach)
			})

			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				connestd := make(chan struct{}, 1)
				offerPC.OnConnectionStateChange(func(state PeerConnectionState) {
					if state == PeerConnectionStateConnected {
						connestd <- struct{}{}
					}
				})
				select {
				case <-connestd:
				case <-time.After(10 * time.Second):
					assert.Fail(t, "conn establishment timed out")

					return
				}
				<-readDetach
				err1 := dc1.dataChannel.SetReadDeadline(time.Now().Add(10 * time.Second))
				assert.NoError(t, err1)
				buf := make([]byte, 10)
				n, err1 := dc1.dataChannel.Read(buf)
				assert.NoError(t, err1)
				assert.Equal(t, "hello", string(buf[:n]), "invalid read")
			}()
			go func() {
				defer wg.Done()
				connestd := make(chan struct{}, 1)
				answerPC.OnConnectionStateChange(func(state PeerConnectionState) {
					if state == PeerConnectionStateConnected {
						connestd <- struct{}{}
					}
				})
				select {
				case <-connestd:
				case <-time.After(10 * time.Second):
					assert.Fail(t, "connection establishment timed out")

					return
				}
				<-writeDetach
				n, err1 := dc2.dataChannel.Write([]byte("hello"))
				assert.NoError(t, err1)
				assert.Equal(t, len("hello"), n)
			}()
			err = signalPair(offerPC, answerPC)
			require.NoError(t, err)
			wg.Wait()
		}()
	}

	for i := 0; i < N; i++ {
		select {
		case <-done:
		case <-time.After(20 * time.Second):
			assert.Fail(t, "timed out")
		}
	}
}

// Assert that max-message-size is signaled properly
// and able to be configured via SettingEngine.
func TestMaxMessageSizeSignaling(t *testing.T) {
	t.Run("Local Offer", func(t *testing.T) {
		peerConnection, err := NewPeerConnection(Configuration{})
		require.NoError(t, err)

		_, err = peerConnection.CreateDataChannel("", nil)
		require.NoError(t, err)

		offer, err := peerConnection.CreateOffer(nil)
		require.NoError(t, err)

		require.Contains(t, offer.SDP, "a=max-message-size:1073741823\r\n")
		require.NoError(t, peerConnection.Close())
	})

	t.Run("Local SettingEngine", func(t *testing.T) {
		settingEngine := SettingEngine{}
		settingEngine.SetSCTPMaxMessageSize(4321)

		peerConnection, err := NewAPI(WithSettingEngine(settingEngine)).NewPeerConnection(Configuration{})
		require.NoError(t, err)

		_, err = peerConnection.CreateDataChannel("", nil)
		require.NoError(t, err)

		offer, err := peerConnection.CreateOffer(nil)
		require.NoError(t, err)

		require.Contains(t, offer.SDP, "a=max-message-size:4321\r\n")
		require.NoError(t, peerConnection.Close())
	})

	t.Run("Remote", func(t *testing.T) {
		settingEngine := SettingEngine{}
		settingEngine.SetSCTPMaxMessageSize(4321)

		offerPeerConnection, err := NewAPI(WithSettingEngine(settingEngine)).NewPeerConnection(Configuration{})
		require.NoError(t, err)

		answerPeerConnection, err := NewPeerConnection(Configuration{})
		require.NoError(t, err)

		onDataChannelOpen, onDataChannelOpenCancel := context.WithCancel(context.Background())
		answerPeerConnection.OnDataChannel(func(d *DataChannel) {
			d.OnOpen(func() {
				onDataChannelOpenCancel()
			})
		})

		require.NoError(t, signalPair(offerPeerConnection, answerPeerConnection))

		<-onDataChannelOpen.Done()
		require.Equal(t, uint32(defaultMaxSCTPMessageSize), offerPeerConnection.SCTP().GetCapabilities().MaxMessageSize)
		require.Equal(t, uint32(4321), answerPeerConnection.SCTP().GetCapabilities().MaxMessageSize)

		closePairNow(t, offerPeerConnection, answerPeerConnection)
	})

	t.Run("Remote Unset", func(t *testing.T) {
		offerPeerConnection, answerPeerConnection, err := newPair()
		require.NoError(t, err)

		require.NoError(t, signalPairWithModification(offerPeerConnection, answerPeerConnection, func(sessionDescription string) (filtered string) { // nolint
			scanner := bufio.NewScanner(strings.NewReader(sessionDescription))
			for scanner.Scan() {
				if strings.HasPrefix(scanner.Text(), "a=max-message-size") {
					continue
				}

				filtered += scanner.Text() + "\r\n"
			}

			return
		}))

		onDataChannelOpen, onDataChannelOpenCancel := context.WithCancel(context.Background())
		answerPeerConnection.OnDataChannel(func(d *DataChannel) {
			d.OnOpen(func() {
				onDataChannelOpenCancel()
			})
		})

		require.NoError(t, signalPair(offerPeerConnection, answerPeerConnection))

		<-onDataChannelOpen.Done()
		require.Equal(t, uint32(defaultMaxSCTPMessageSize), offerPeerConnection.SCTP().GetCapabilities().MaxMessageSize)
		require.Equal(t, uint32(sctpMaxMessageSizeUnsetValue), answerPeerConnection.SCTP().GetCapabilities().MaxMessageSize)

		closePairNow(t, offerPeerConnection, answerPeerConnection)
	})
}
