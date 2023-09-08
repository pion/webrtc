// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"regexp"
	"testing"
	"time"

	"github.com/pion/transport/v2/test"
	"github.com/stretchr/testify/assert"
)

// An invalid fingerprint MUST cause PeerConnectionState to go to PeerConnectionStateFailed
func TestInvalidFingerprintCausesFailed(t *testing.T) {
	lim := test.TimeOut(time.Second * 40)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	pcAnswer, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	pcAnswer.OnDataChannel(func(_ *DataChannel) {
		t.Fatal("A DataChannel must not be created when Fingerprint verification fails")
	})

	defer closePairNow(t, pcOffer, pcAnswer)

	offerChan := make(chan SessionDescription)
	pcOffer.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			offerChan <- *pcOffer.PendingLocalDescription()
		}
	})

	offerConnectionHasFailed := untilConnectionState(PeerConnectionStateFailed, pcOffer)
	answerConnectionHasFailed := untilConnectionState(PeerConnectionStateFailed, pcAnswer)

	if _, err = pcOffer.CreateDataChannel("unusedDataChannel", nil); err != nil {
		t.Fatal(err)
	}

	offer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	} else if err := pcOffer.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}

	select {
	case offer := <-offerChan:
		// Replace with invalid fingerprint
		re := regexp.MustCompile(`sha-256 (.*?)\r`)
		offer.SDP = re.ReplaceAllString(offer.SDP, "sha-256 AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA\r")

		if err := pcAnswer.SetRemoteDescription(offer); err != nil {
			t.Fatal(err)
		}

		answer, err := pcAnswer.CreateAnswer(nil)
		if err != nil {
			t.Fatal(err)
		}

		if err = pcAnswer.SetLocalDescription(answer); err != nil {
			t.Fatal(err)
		}

		answer.SDP = re.ReplaceAllString(answer.SDP, "sha-256 AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA\r")

		err = pcOffer.SetRemoteDescription(answer)
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting to receive offer")
	}

	offerConnectionHasFailed.Wait()
	answerConnectionHasFailed.Wait()

	assert.Equal(t, pcOffer.SCTP().Transport().State(), DTLSTransportStateFailed)
	assert.Nil(t, pcOffer.SCTP().Transport().conn)

	assert.Equal(t, pcAnswer.SCTP().Transport().State(), DTLSTransportStateFailed)
	assert.Nil(t, pcAnswer.SCTP().Transport().conn)
}

func TestPeerConnection_DTLSRoleSettingEngine(t *testing.T) {
	runTest := func(r DTLSRole) {
		s := SettingEngine{}
		assert.NoError(t, s.SetAnsweringDTLSRole(r))

		offerPC, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
		if err != nil {
			t.Fatal(err)
		}

		answerPC, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
		if err != nil {
			t.Fatal(err)
		}

		if err = signalPair(offerPC, answerPC); err != nil {
			t.Fatal(err)
		}

		connectionComplete := untilConnectionState(PeerConnectionStateConnected, answerPC)
		connectionComplete.Wait()
		closePairNow(t, offerPC, answerPC)
	}

	report := test.CheckRoutines(t)
	defer report()

	t.Run("Server", func(t *testing.T) {
		runTest(DTLSRoleServer)
	})

	t.Run("Client", func(t *testing.T) {
		runTest(DTLSRoleClient)
	})
}
