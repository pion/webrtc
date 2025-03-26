// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"regexp"
	"testing"
	"time"

	"github.com/pion/transport/v3/test"
	"github.com/stretchr/testify/assert"
)

// An invalid fingerprint MUST cause PeerConnectionState to go to PeerConnectionStateFailed.
func TestInvalidFingerprintCausesFailed(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 5)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pcAnswer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pcAnswer.OnDataChannel(func(_ *DataChannel) {
		assert.Fail(t, "A DataChannel must not be created when Fingerprint verification fails")
	})

	defer closePairNow(t, pcOffer, pcAnswer)

	offerChan := make(chan SessionDescription)
	pcOffer.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			offerChan <- *pcOffer.PendingLocalDescription()
		}
	})

	offerConnectionHasClosed := untilConnectionState(PeerConnectionStateClosed, pcOffer)
	answerConnectionHasClosed := untilConnectionState(PeerConnectionStateClosed, pcAnswer)

	_, err = pcOffer.CreateDataChannel("unusedDataChannel", nil)
	assert.NoError(t, err)

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))

	select {
	case offer := <-offerChan:
		// Replace with invalid fingerprint
		re := regexp.MustCompile(`sha-256 (.*?)\r`)
		offer.SDP = re.ReplaceAllString(
			offer.SDP,
			"sha-256 AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA\r",
		)

		assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

		answer, err := pcAnswer.CreateAnswer(nil)
		assert.NoError(t, err)
		assert.NoError(t, pcAnswer.SetLocalDescription(answer))

		answer.SDP = re.ReplaceAllString(
			answer.SDP,
			"sha-256 AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA\r",
		)

		assert.NoError(t, pcOffer.SetRemoteDescription(answer))
	case <-time.After(5 * time.Second):
		assert.Fail(t, "timed out waiting to receive offer")
	}

	offerConnectionHasClosed.Wait()
	answerConnectionHasClosed.Wait()

	assert.Contains(
		t, []DTLSTransportState{DTLSTransportStateClosed, DTLSTransportStateFailed}, pcOffer.SCTP().Transport().State(),
		"DTLS Transport should be closed or failed",
	)
	assert.Nil(t, pcOffer.SCTP().Transport().conn)

	assert.Contains(
		t, []DTLSTransportState{DTLSTransportStateClosed, DTLSTransportStateFailed}, pcAnswer.SCTP().Transport().State(),
		"DTLS Transport should be closed or failed",
	)
	assert.Nil(t, pcAnswer.SCTP().Transport().conn)
}

func TestPeerConnection_DTLSRoleSettingEngine(t *testing.T) {
	runTest := func(r DTLSRole) {
		s := SettingEngine{}
		assert.NoError(t, s.SetAnsweringDTLSRole(r))

		offerPC, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		answerPC, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
		assert.NoError(t, err)
		assert.NoError(t, signalPair(offerPC, answerPC))

		connectionComplete := untilConnectionState(PeerConnectionStateConnected, answerPC)
		connectionComplete.Wait()
		closePairNow(t, offerPC, answerPC)
	}

	report := test.CheckRoutines(t)
	defer report()

	t.Run("Server", func(*testing.T) {
		runTest(DTLSRoleServer)
	})

	t.Run("Client", func(*testing.T) {
		runTest(DTLSRoleClient)
	})
}
