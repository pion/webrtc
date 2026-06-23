// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/transport/v4/test"
	"github.com/stretchr/testify/assert"
)

func TestICETransport_StartContextClosesOnCancel(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	api := NewAPI()
	gatherer, err := api.NewICEGatherer(ICEGatherOptions{})
	assert.NoError(t, err)

	remoteGatherer, err := api.NewICEGatherer(ICEGatherOptions{})
	assert.NoError(t, err)

	params, err := remoteGatherer.GetLocalParameters()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, remoteGatherer.Close())
	}()

	transport := api.NewICETransport(gatherer)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	controlling := ICERoleControlling
	err = transport.StartContext(ctx, nil, params, &controlling)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, ICEGathererStateClosed, gatherer.State())
	assert.Nil(t, gatherer.getAgent())
}

func TestICETransport_StartContextClearsCancelOnRoleError(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	api := NewAPI()
	gatherer, err := api.NewICEGatherer(ICEGatherOptions{})
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, gatherer.Close())
	}()

	remoteGatherer, err := api.NewICEGatherer(ICEGatherOptions{})
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, remoteGatherer.Close())
	}()

	params, err := remoteGatherer.GetLocalParameters()
	assert.NoError(t, err)

	transport := api.NewICETransport(gatherer)
	role := ICERoleUnknown
	err = transport.StartContext(context.Background(), nil, params, &role)
	assert.ErrorIs(t, err, errICERoleUnknown)
	assert.Nil(t, transport.ctxCancel)
	assert.NotEqual(t, ICEGathererStateClosed, gatherer.State())
	assert.NotNil(t, gatherer.getAgent())
}

func TestICETransport_StartContextStopDoesNotReportCallerCancel(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	api := NewAPI()
	gatherer, err := api.NewICEGatherer(ICEGatherOptions{})
	assert.NoError(t, err)

	remoteGatherer, err := api.NewICEGatherer(ICEGatherOptions{})
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, remoteGatherer.Close())
	}()

	params, err := remoteGatherer.GetLocalParameters()
	assert.NoError(t, err)

	transport := api.NewICETransport(gatherer)
	controlling := ICERoleControlling
	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.StartContext(context.Background(), nil, params, &controlling)
	}()

	if !assert.Eventually(t, func() bool {
		transport.lock.RLock()
		defer transport.lock.RUnlock()

		return transport.ctxCancel != nil
	}, time.Second, time.Millisecond) {
		return
	}

	assert.NoError(t, transport.Stop())

	select {
	case err = <-errCh:
		assert.Error(t, err)
		assert.NotErrorIs(t, err, context.Canceled)
		assert.True(t,
			errors.Is(err, ice.ErrCanceledByCaller) || errors.Is(err, ice.ErrClosed),
			"expected ICE shutdown error, got %v",
			err,
		)
	case <-time.After(time.Second):
		t.Fatal("StartContext did not return after Stop")
	}
}

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
	pcOffer.SCTP().Transport().ICETransport().OnSelectedCandidatePairChange(func(*ICECandidatePair) {
		atomic.StoreInt32(&senderCalledCandidateChange, 1)
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))
	<-iceComplete

	assert.NotEmpty(
		t, atomic.LoadInt32(&senderCalledCandidateChange),
		"Sender ICETransport OnSelectedCandidateChange was never called",
	)

	closePairNow(t, pcOffer, pcAnswer)
}

func TestICETransport_GetSelectedCandidatePair(t *testing.T) {
	offerer, answerer, err := newPair()
	assert.NoError(t, err)

	peerConnectionConnected := untilConnectionState(PeerConnectionStateConnected, offerer, answerer)

	offererSelectedPair, err := offerer.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
	assert.NoError(t, err)
	assert.Nil(t, offererSelectedPair)
	_, statsAvailable := offerer.SCTP().Transport().ICETransport().GetSelectedCandidatePairStats()
	assert.False(t, statsAvailable)

	answererSelectedPair, err := answerer.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
	assert.NoError(t, err)
	assert.Nil(t, answererSelectedPair)
	_, statsAvailable = answerer.SCTP().Transport().ICETransport().GetSelectedCandidatePairStats()
	assert.False(t, statsAvailable)

	assert.NoError(t, signalPair(offerer, answerer))
	peerConnectionConnected.Wait()

	offererSelectedPair, err = offerer.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
	assert.NoError(t, err)
	assert.NotNil(t, offererSelectedPair)
	_, statsAvailable = offerer.SCTP().Transport().ICETransport().GetSelectedCandidatePairStats()
	assert.True(t, statsAvailable)

	answererSelectedPair, err = answerer.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
	assert.NoError(t, err)
	assert.NotNil(t, answererSelectedPair)
	_, statsAvailable = answerer.SCTP().Transport().ICETransport().GetSelectedCandidatePairStats()
	assert.True(t, statsAvailable)

	closePairNow(t, offerer, answerer)
}

func TestICETransport_GetLocalAndRemoteParameters(t *testing.T) {
	offerer, answerer, err := newPair()
	assert.NoError(t, err)

	_, err = offerer.SCTP().Transport().ICETransport().GetRemoteParameters()
	assert.Error(t, err, errICEAgentNotExist)

	peerConnectionConnected := untilConnectionState(PeerConnectionStateConnected, offerer, answerer)

	assert.NoError(t, signalPair(offerer, answerer))
	peerConnectionConnected.Wait()

	offerLocalParameters, err := offerer.SCTP().Transport().ICETransport().GetLocalParameters()
	assert.NoError(t, err)

	offerRemoteParameters, err := offerer.SCTP().Transport().ICETransport().GetRemoteParameters()
	assert.NoError(t, err)

	answerLocalParameters, err := answerer.SCTP().Transport().ICETransport().GetLocalParameters()
	assert.NoError(t, err)

	answerRemoteParameters, err := answerer.SCTP().Transport().ICETransport().GetRemoteParameters()
	assert.NoError(t, err)

	assert.Equal(t, offerLocalParameters.UsernameFragment, answerRemoteParameters.UsernameFragment)
	assert.Equal(t, offerLocalParameters.Password, answerRemoteParameters.Password)
	assert.Equal(t, answerLocalParameters.UsernameFragment, offerRemoteParameters.UsernameFragment)
	assert.Equal(t, answerLocalParameters.Password, offerRemoteParameters.Password)

	closePairNow(t, offerer, answerer)
}
