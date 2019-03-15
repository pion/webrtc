package webrtc

import (
	"io"
	"testing"
	"time"
)

func closePair(t *testing.T, pc1, pc2 io.Closer, done chan bool) {
	var err error
	select {
	case <-time.After(10 * time.Second):
		t.Fatalf("closePair timed out waiting for done signal")
	case <-done:
		err = pc1.Close()
		if err != nil {
			t.Fatalf("Failed to close offer PC")
		}
		err = pc2.Close()
		if err != nil {
			t.Fatalf("Failed to close answer PC")
		}
	}
}

func setUpReliabilityParamTest(t *testing.T, options *DataChannelInit) (*PeerConnection, *PeerConnection, *DataChannel, chan bool) {
	offerPC, answerPC, err := newPair()
	if err != nil {
		t.Fatalf("Failed to create a PC pair for testing")
	}
	done := make(chan bool)

	dc, err := offerPC.CreateDataChannel("data", options)
	if err != nil {
		t.Fatalf("Failed to create a PC pair for testing")
	}

	return offerPC, answerPC, dc, done
}

func closeReliabilityParamTest(t *testing.T, pc1, pc2 *PeerConnection, done chan bool) {
	err := signalPair(pc1, pc2)
	if err != nil {
		t.Fatalf("Failed to signal our PC pair for testing")
	}

	closePair(t, pc1, pc2, done)
}
