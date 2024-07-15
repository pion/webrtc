// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"testing"
	"time"

	"github.com/pion/transport/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestPeerConnection_Close(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

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
	if err != nil {
		t.Fatal(err)
	}

	err = signalPair(pcOffer, pcAnswer)
	if err != nil {
		t.Fatal(err)
	}

	<-awaitSetup

	closePairNow(t, pcOffer, pcAnswer)

	<-awaitICEClosed
}

// Assert that a PeerConnection that is shutdown before ICE starts doesn't leak
func TestPeerConnection_Close_PreICE(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcOffer.CreateDataChannel("test-channel", nil)
	if err != nil {
		t.Fatal(err)
	}

	answer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}

	assert.NoError(t, pcOffer.Close())

	if err = pcAnswer.SetRemoteDescription(answer); err != nil {
		t.Fatal(err)
	}

	for {
		if pcAnswer.iceTransport.State() == ICETransportStateChecking {
			break
		}
		time.Sleep(time.Second / 4)
	}

	assert.NoError(t, pcAnswer.Close())

	// Assert that ICETransport is shutdown, test timeout will prevent deadlock
	for {
		if pcAnswer.iceTransport.State() == ICETransportStateClosed {
			return
		}
		time.Sleep(time.Second / 4)
	}
}

func TestPeerConnection_Close_DuringICE(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatal(err)
	}

	offer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}

	offerGatheringComplete := GatheringCompletePromise(pcOffer)
	if err = pcOffer.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	<-offerGatheringComplete

	if err = pcAnswer.SetRemoteDescription(*pcOffer.LocalDescription()); err != nil {
		t.Fatal(err)
	}

	answer, err := pcAnswer.CreateAnswer(nil)
	if err != nil {
		t.Fatal(err)
	}
	answerGatheringComplete := GatheringCompletePromise(pcAnswer)
	if err = pcAnswer.SetLocalDescription(answer); err != nil {
		t.Fatal(err)
	}
	<-answerGatheringComplete
	if err = pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()); err != nil {
		t.Fatal(err)
	}

	select {
	case <-closedAnswer:
	case <-time.After(5 * time.Second):
		t.Error("pcAnswer.Close() Timeout")
	}
	select {
	case <-closedOffer:
	case <-time.After(5 * time.Second):
		t.Error("pcOffer.Close() Timeout")
	}
}
