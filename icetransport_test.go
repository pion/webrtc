// +build !js

package webrtc

import (
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pions/transport/test"
)

func TestICETransport_OnSelectedCandidatePairChange(t *testing.T) {
	iceComplete := make(chan bool)

	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

	opusTrack, err := pcOffer.NewTrack(DefaultPayloadTypeOpus, rand.Uint32(), "audio", "pion1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pcOffer.AddTrack(opusTrack); err != nil {
		t.Fatal(err)
	}

	pcAnswer.OnICEConnectionStateChange(func(iceState ICEConnectionState) {
		if iceState == ICEConnectionStateConnected {
			time.Sleep(3 * time.Second) // TODO PeerConnection.Close() doesn't block for all subsystems
			close(iceComplete)
		}
	})

	senderCalledCandidateChange := int32(0)
	for _, sender := range pcOffer.GetSenders() {
		dtlsTransport := sender.Transport()
		if dtlsTransport == nil {
			continue
		}
		if iceTransport := dtlsTransport.ICETransport(); iceTransport != nil {
			iceTransport.OnSelectedCandidatePairChange(func(pair *ICECandidatePair) {
				atomic.StoreInt32(&senderCalledCandidateChange, 1)
			})
		}
	}

	err = signalPair(pcOffer, pcAnswer)
	if err != nil {
		t.Fatal(err)
	}
	<-iceComplete

	if atomic.LoadInt32(&senderCalledCandidateChange) == 0 {
		t.Fatalf("Sender ICETransport OnSelectedCandidateChange was never called")
	}

	err = pcOffer.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = pcAnswer.Close()
	if err != nil {
		t.Fatal(err)
	}
}
