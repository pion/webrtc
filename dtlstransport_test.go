// +build !js

package webrtc

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/pion/transport/test"
	"github.com/stretchr/testify/assert"
)

// An invalid fingerprint MUST cause PeerConnectionState to go to PeerConnectionStateFailed
func TestInvalidFingerprintCausesFailed(t *testing.T) {
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

	defer closePairNow(t, pcOffer, pcAnswer)

	offerChan := make(chan SessionDescription)
	pcOffer.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			offerChan <- *pcOffer.PendingLocalDescription()
		}
	})

	connectionHasFailed, closeFunc := context.WithCancel(context.Background())
	pcAnswer.OnConnectionStateChange(func(connectionState PeerConnectionState) {
		if connectionState == PeerConnectionStateFailed {
			closeFunc()
		}
	})

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

		err = pcOffer.SetRemoteDescription(answer)
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting to receive offer")
	}

	select {
	case <-connectionHasFailed.Done():
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for connection to fail")
	}
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

		connectionComplete := make(chan interface{})
		answerPC.OnConnectionStateChange(func(connectionState PeerConnectionState) {
			if connectionState == PeerConnectionStateConnected {
				select {
				case <-connectionComplete:
				default:
					close(connectionComplete)
				}
			}
		})

		<-connectionComplete
		assert.NoError(t, offerPC.Close())
		assert.NoError(t, answerPC.Close())
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
