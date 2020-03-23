// +build !js

package webrtc

import (
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/transport/test"
	"github.com/stretchr/testify/assert"
)

func TestICETransport_OnSelectedCandidatePairChange(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	api := NewAPI()
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

	iceComplete := make(chan bool)
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

	assert.NoError(t, signalPair(pcOffer, pcAnswer))
	<-iceComplete

	if atomic.LoadInt32(&senderCalledCandidateChange) == 0 {
		t.Fatalf("Sender ICETransport OnSelectedCandidateChange was never called")
	}

	closePairNow(t, pcOffer, pcAnswer)
}
