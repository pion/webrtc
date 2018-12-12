package webrtc

import (
	"testing"
	"time"

	"github.com/pions/transport/test"
)

// TestRTCPeerConnection_Close is moved to it's on file because the tests
// in rtcpeerconnection_test.go are leaky, making the goroutine report useless.

func TestRTCPeerConnection_Close(t *testing.T) {
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
	pcAnswer.OnDataChannel(func(d *RTCDataChannel) {
		close(awaitSetup)
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

	err = pcOffer.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = pcAnswer.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func newPair() (pcOffer *RTCPeerConnection, pcAnswer *RTCPeerConnection, err error) {
	pca, err := New(RTCConfiguration{})
	if err != nil {
		return nil, nil, err
	}

	pcb, err := New(RTCConfiguration{})
	if err != nil {
		return nil, nil, err
	}

	return pca, pcb, nil
}

func signalPair(pcOffer *RTCPeerConnection, pcAnswer *RTCPeerConnection) error {
	offer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		return err
	}

	err = pcAnswer.SetRemoteDescription(offer)
	if err != nil {
		return err
	}

	answer, err := pcAnswer.CreateAnswer(nil)
	if err != nil {
		return err
	}

	err = pcOffer.SetRemoteDescription(answer)
	if err != nil {
		return err
	}

	return nil
}
