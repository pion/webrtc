// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"regexp"
	"testing"
	"time"

	"github.com/pion/transport/v4/test"
	"github.com/stretchr/testify/assert"
)

// An invalid fingerprint MUST cause DTLSTransport to go to failed state.
func TestInvalidFingerprintCausesFailed(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 10)
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

	// Set up DTLS state tracking BEFORE starting the connection process
	// to avoid missing the state transition
	offerDTLSFailed := make(chan struct{})
	answerDTLSFailed := make(chan struct{})
	pcOffer.SCTP().Transport().OnStateChange(func(state DTLSTransportState) {
		if state == DTLSTransportStateFailed {
			select {
			case <-offerDTLSFailed:
				// Already closed
			default:
				close(offerDTLSFailed)
			}
		}
	})
	pcAnswer.SCTP().Transport().OnStateChange(func(state DTLSTransportState) {
		if state == DTLSTransportStateFailed {
			select {
			case <-answerDTLSFailed:
				// Already closed
			default:
				close(answerDTLSFailed)
			}
		}
	})

	offerChan := make(chan SessionDescription)
	pcOffer.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			offerChan <- *pcOffer.PendingLocalDescription()
		}
	})

	// Also wait for PeerConnection to close (may take longer due to cleanup)
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

	// Wait for DTLS to fail (should happen quickly after ICE connects, ~1-2 seconds normally,
	// but may take longer with race detector due to ICE connectivity checks)
	select {
	case <-offerDTLSFailed:
		// Expected - offer DTLS failed due to invalid fingerprint
	case <-time.After(7 * time.Second):
		assert.Fail(t, "timed out waiting for offer DTLS to fail")
	}

	select {
	case <-answerDTLSFailed:
		// Expected - answer DTLS failed due to invalid fingerprint
	case <-time.After(7 * time.Second):
		assert.Fail(t, "timed out waiting for answer DTLS to fail")
	}

	// Wait for PeerConnection to close (may take longer due to cleanup)
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
