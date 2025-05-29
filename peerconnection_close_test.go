// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pion/transport/v3/test"
	"github.com/stretchr/testify/assert"
)

func TestPeerConnection_Close(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	awaitSetup := make(chan struct{})
	pcAnswer.OnDataChannel(func(d *DataChannel) {
		// Make sure this is the data channel we were looking for. (Not the one
		// created in signalPair).
		if d.Label() != "data" {
			return
		}
		close(awaitSetup)
	})

	awaitICEClosed := make(chan struct{})
	pcAnswer.OnICEConnectionStateChange(func(i ICEConnectionState) {
		if i == ICEConnectionStateClosed {
			close(awaitICEClosed)
		}
	})

	_, err = pcOffer.CreateDataChannel("data", nil)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	<-awaitSetup

	closePairNow(t, pcOffer, pcAnswer)

	<-awaitICEClosed
}

// Assert that a PeerConnection that is shutdown before ICE starts doesn't leak.
func TestPeerConnection_Close_PreICE(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	_, err = pcOffer.CreateDataChannel("test-channel", nil)
	assert.NoError(t, err)

	answer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	assert.NoError(t, pcOffer.Close())

	assert.NoError(t, pcAnswer.SetRemoteDescription(answer))

	for pcAnswer.iceTransport.State() != ICETransportStateChecking {
		time.Sleep(time.Second / 4)
	}

	assert.NoError(t, pcAnswer.Close())

	// Assert that ICETransport is shutdown, test timeout will prevent deadlock
	for pcAnswer.iceTransport.State() != ICETransportStateClosed {
		time.Sleep(time.Second / 4)
	}
}

func TestPeerConnection_Close_DuringICE(t *testing.T) { //nolint:cyclop
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)
	closedOffer := make(chan struct{})
	closedAnswer := make(chan struct{})
	pcAnswer.OnICEConnectionStateChange(func(iceState ICEConnectionState) {
		if iceState == ICEConnectionStateConnected {
			go func() {
				assert.NoError(t, pcAnswer.Close())
				close(closedAnswer)

				assert.NoError(t, pcOffer.Close())
				close(closedOffer)
			}()
		}
	})

	_, err = pcOffer.CreateDataChannel("test-channel", nil)
	assert.NoError(t, err)

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	offerGatheringComplete := GatheringCompletePromise(pcOffer)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))
	<-offerGatheringComplete

	assert.NoError(t, pcAnswer.SetRemoteDescription(*pcOffer.LocalDescription()))

	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)
	answerGatheringComplete := GatheringCompletePromise(pcAnswer)
	assert.NoError(t, pcAnswer.SetLocalDescription(answer))
	<-answerGatheringComplete
	assert.NoError(t, pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()))

	select {
	case <-closedAnswer:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "pcAnswer.Close() Timeout")
	}
	select {
	case <-closedOffer:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "pcOffer.Close() Timeout")
	}
}

func TestPeerConnection_GracefulCloseWithIncomingMessages(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutinesStrict(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	var dcAnswer *DataChannel
	answerDataChannelOpened := make(chan struct{})
	pcAnswer.OnDataChannel(func(d *DataChannel) {
		// Make sure this is the data channel we were looking for. (Not the one
		// created in signalPair).
		if d.Label() != "data" {
			return
		}
		dcAnswer = d
		close(answerDataChannelOpened)
	})

	dcOffer, err := pcOffer.CreateDataChannel("data", nil)
	assert.NoError(t, err)

	offerDataChannelOpened := make(chan struct{})
	dcOffer.OnOpen(func() {
		close(offerDataChannelOpened)
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	<-offerDataChannelOpened
	<-answerDataChannelOpened

	msgNum := 0
	dcOffer.OnMessage(func(_ DataChannelMessage) {
		t.Log("msg", msgNum)
		msgNum++
	})

	// send 50 messages, then close pcOffer, and then send another 50
	for i := 0; i < 100; i++ {
		if i == 50 {
			assert.NoError(t, pcOffer.GracefulClose())
		}
		_ = dcAnswer.Send([]byte("hello!"))
	}

	assert.NoError(t, pcAnswer.GracefulClose())
}

func TestPeerConnection_GracefulCloseWhileOpening(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 5)
	defer lim.Stop()

	report := test.CheckRoutinesStrict(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	_, err = pcOffer.CreateDataChannel("initial_data_channel", nil)
	assert.NoError(t, err)

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)
	offerGatheringComplete := GatheringCompletePromise(pcOffer)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))
	<-offerGatheringComplete

	assert.NoError(t, pcOffer.GracefulClose())

	assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

	err = pcAnswer.GracefulClose()
	assert.NoError(t, err)
}

func TestPeerConnection_GracefulCloseConcurrent(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprintf("mixed_graceful=%t", mixed), func(t *testing.T) {
			report := test.CheckRoutinesStrict(t)
			defer report()

			pc, err := NewPeerConnection(Configuration{})
			assert.NoError(t, err)

			const gracefulCloseConcurrency = 50
			var wg sync.WaitGroup
			wg.Add(gracefulCloseConcurrency)
			for i := 0; i < gracefulCloseConcurrency; i++ {
				go func() {
					defer wg.Done()
					assert.NoError(t, pc.GracefulClose())
				}()
			}
			if !mixed {
				assert.NoError(t, pc.Close())
			} else {
				assert.NoError(t, pc.GracefulClose())
			}
			wg.Wait()
		})
	}
}
