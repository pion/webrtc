// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/transport/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestICETransport_OnConnectionStateChange(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	var (
		iceComplete             sync.WaitGroup
		peerConnectionConnected sync.WaitGroup
	)
	iceComplete.Add(2)
	peerConnectionConnected.Add(2)

	onIceComplete := func(s ICETransportState) {
		if s == ICETransportStateConnected {
			iceComplete.Done()
		}
	}
	pcOffer.SCTP().Transport().ICETransport().OnConnectionStateChange(onIceComplete)
	pcAnswer.SCTP().Transport().ICETransport().OnConnectionStateChange(onIceComplete)

	onConnected := func(s PeerConnectionState) {
		if s == PeerConnectionStateConnected {
			peerConnectionConnected.Done()
		}
	}
	pcOffer.OnConnectionStateChange(onConnected)
	pcAnswer.OnConnectionStateChange(onConnected)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))
	iceComplete.Wait()
	peerConnectionConnected.Wait()

	closePairNow(t, pcOffer, pcAnswer)
}

func TestICETransport_OnSelectedCandidatePairChange(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	iceComplete := make(chan bool)
	pcAnswer.OnICEConnectionStateChange(func(iceState ICEConnectionState) {
		if iceState == ICEConnectionStateConnected {
			time.Sleep(3 * time.Second)
			close(iceComplete)
		}
	})

	senderCalledCandidateChange := int32(0)
	pcOffer.SCTP().Transport().ICETransport().OnSelectedCandidatePairChange(func(pair *ICECandidatePair) {
		atomic.StoreInt32(&senderCalledCandidateChange, 1)
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))
	<-iceComplete

	if atomic.LoadInt32(&senderCalledCandidateChange) == 0 {
		t.Fatalf("Sender ICETransport OnSelectedCandidateChange was never called")
	}

	closePairNow(t, pcOffer, pcAnswer)
}

func TestICETransport_GetSelectedCandidatePair(t *testing.T) {
	offerer, answerer, err := newPair()
	assert.NoError(t, err)

	peerConnectionConnected := untilConnectionState(PeerConnectionStateConnected, offerer, answerer)

	offererSelectedPair, err := offerer.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
	assert.NoError(t, err)
	assert.Nil(t, offererSelectedPair)

	answererSelectedPair, err := answerer.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
	assert.NoError(t, err)
	assert.Nil(t, answererSelectedPair)

	assert.NoError(t, signalPair(offerer, answerer))
	peerConnectionConnected.Wait()

	offererSelectedPair, err = offerer.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
	assert.NoError(t, err)
	assert.NotNil(t, offererSelectedPair)

	answererSelectedPair, err = answerer.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
	assert.NoError(t, err)
	assert.NotNil(t, answererSelectedPair)

	closePairNow(t, offerer, answerer)
}

func TestICETransport_GetLocalParameters(t *testing.T) {
	offerer, answerer, err := newPair()
	assert.NoError(t, err)

	peerConnectionConnected := untilConnectionState(PeerConnectionStateConnected, offerer, answerer)

	assert.NoError(t, signalPair(offerer, answerer))
	peerConnectionConnected.Wait()

	localParameters, err := offerer.SCTP().Transport().ICETransport().GetLocalParameters()
	assert.NoError(t, err)
	assert.NotEqual(t, localParameters.UsernameFragment, "")
	assert.NotEqual(t, localParameters.Password, "")

	closePairNow(t, offerer, answerer)
}
