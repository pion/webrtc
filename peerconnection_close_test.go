// +build !js

package webrtc

import (
	"testing"
	"time"

	"github.com/pion/transport/test"
)

// TestPeerConnection_Close is moved to it's own file because the tests
// in rtcpeerconnection_test.go are leaky, making the goroutine report useless.

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

	answer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}

	err = pcOffer.Close()
	if err != nil {
		t.Fatal(err)
	}

	if err = pcAnswer.SetRemoteDescription(answer); err != nil {
		t.Fatal(err)
	}

	for {
		if pcAnswer.iceTransport.State() == ICETransportStateChecking {
			break
		}
		time.Sleep(time.Second)
	}

	err = pcAnswer.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Assert that ICETransport is shutdown, test timeout will prevent deadlock
	for {
		if pcAnswer.iceTransport.State() == ICETransportStateClosed {
			time.Sleep(time.Second * 3)
			return
		}

		time.Sleep(time.Second)
	}
}
