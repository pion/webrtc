package webrtc

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/pions/rtcp"
	"github.com/pions/transport/test"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/media"
)

func TestPeerConnection_Media_Sample(t *testing.T) {
	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

	awaitRTPRecv := make(chan bool)
	awaitRTPRecvClosed := make(chan bool)
	awaitRTPSend := make(chan bool)

	awaitRTCPSenderRecv := make(chan bool)
	awaitRTCPSenderSend := make(chan error)

	awaitRTCPRecieverRecv := make(chan bool)
	awaitRTCPRecieverSend := make(chan error)

	pcAnswer.OnTrack(func(track *Track) {
		go func() {
			for {
				time.Sleep(time.Millisecond * 100)
				if routineErr := pcAnswer.SendRTCP(&rtcp.RapidResynchronizationRequest{SenderSSRC: track.SSRC, MediaSSRC: track.SSRC}); routineErr != nil {
					awaitRTCPRecieverSend <- routineErr
					return
				}

				select {
				case <-awaitRTCPSenderRecv:
					close(awaitRTCPRecieverSend)
					return
				default:
				}
			}
		}()

		go func() {
			<-track.RTCPPackets
			close(awaitRTCPRecieverRecv)
		}()

		haveClosedAwaitRTPRecv := false
		for {
			p, ok := <-track.Packets
			if !ok {
				close(awaitRTPRecvClosed)
				return
			} else if bytes.Equal(p.Payload, []byte{0x10, 0x00}) && !haveClosedAwaitRTPRecv {
				haveClosedAwaitRTPRecv = true
				close(awaitRTPRecv)
			}
		}
	})

	vp8Track, err := pcOffer.NewSampleTrack(DefaultPayloadTypeVP8, "video", "pion")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pcOffer.AddTrack(vp8Track); err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			time.Sleep(time.Millisecond * 100)
			vp8Track.Samples <- media.Sample{Data: []byte{0x00}, Samples: 1}

			select {
			case <-awaitRTPRecv:
				close(awaitRTPSend)
				return
			default:
			}
		}
	}()

	go func() {
		for {
			time.Sleep(time.Millisecond * 100)
			if routineErr := pcOffer.SendRTCP(&rtcp.PictureLossIndication{SenderSSRC: vp8Track.SSRC, MediaSSRC: vp8Track.SSRC}); routineErr != nil {
				awaitRTCPSenderSend <- routineErr
			}

			select {
			case <-awaitRTCPRecieverRecv:
				close(awaitRTCPSenderSend)
				return
			default:
			}
		}
	}()

	go func() {
		<-vp8Track.RTCPPackets
		close(awaitRTCPSenderRecv)
	}()

	err = signalPair(pcOffer, pcAnswer)
	if err != nil {
		t.Fatal(err)
	}

	<-awaitRTPRecv
	<-awaitRTPSend

	<-awaitRTCPSenderRecv
	err, ok := <-awaitRTCPSenderSend
	if ok {
		t.Fatal(err)
	}

	<-awaitRTCPRecieverRecv
	err, ok = <-awaitRTCPRecieverSend
	if ok {
		t.Fatal(err)
	}

	err = pcOffer.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = pcAnswer.Close()
	if err != nil {
		t.Fatal(err)
	}

	<-awaitRTPRecvClosed
}

/*
PeerConnection should be able to be torn down at anytime
This test adds an input track and asserts

* OnTrack doesn't fire since no video packets will arrive
* No goroutine leaks
* No deadlocks on shutdown
*/
func TestPeerConnection_Media_Shutdown(t *testing.T) {
	iceComplete := make(chan bool)

	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

	opusTrack, err := pcOffer.NewSampleTrack(DefaultPayloadTypeOpus, "audio", "pion1")
	if err != nil {
		t.Fatal(err)
	}
	vp8Track, err := pcOffer.NewSampleTrack(DefaultPayloadTypeVP8, "video", "pion2")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = pcOffer.AddTrack(opusTrack); err != nil {
		t.Fatal(err)
	} else if _, err = pcOffer.AddTrack(vp8Track); err != nil {
		t.Fatal(err)
	}

	var onTrackFiredLock sync.RWMutex
	onTrackFired := false

	pcAnswer.OnTrack(func(track *Track) {
		onTrackFiredLock.Lock()
		defer onTrackFiredLock.Unlock()
		onTrackFired = true
	})

	pcAnswer.OnICEConnectionStateChange(func(iceState ice.ConnectionState) {
		if iceState == ice.ConnectionStateConnected {
			go func() {
				time.Sleep(3 * time.Second) // TODO PeerConnection.Close() doesn't block for all subsystems
				close(iceComplete)
			}()
		}
	})

	err = signalPair(pcOffer, pcAnswer)
	if err != nil {
		t.Fatal(err)
	}

	<-iceComplete

	err = pcOffer.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = pcAnswer.Close()
	if err != nil {
		t.Fatal(err)
	}

	onTrackFiredLock.Lock()
	if onTrackFired {
		t.Fatalf("PeerConnection OnTrack fired even though we got no packets")
	}
	onTrackFiredLock.Unlock()

}
