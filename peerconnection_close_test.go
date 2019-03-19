package webrtc

import (
	"testing"
	"time"

	"github.com/pions/transport/test"
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
