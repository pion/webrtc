package webrtc

import (
	"bytes"
	"testing"
	"time"

	"github.com/pions/transport/test"
	"github.com/pions/webrtc/pkg/media"
	"github.com/pions/webrtc/pkg/rtcp"
)

func TestRTCPeerConnection_Media_Sample(t *testing.T) {
	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

	awaitRTPRecv := make(chan bool)
	awaitRTPSend := make(chan bool)

	awaitRTCPSenderRecv := make(chan bool)
	awaitRTCPSenderSend := make(chan error)

	awaitRTCPRecieverRecv := make(chan bool)
	awaitRTCPRecieverSend := make(chan error)

	pcAnswer.OnTrack(func(track *RTCTrack) {
		go func() {
			for {
				time.Sleep(time.Millisecond * 100)
				if routineErr := pcAnswer.SendRTCP(&rtcp.PictureLossIndication{SenderSSRC: track.Ssrc, MediaSSRC: track.Ssrc}); routineErr != nil {
					awaitRTCPRecieverSend <- routineErr
					return
				}

				select {
				case <-awaitRTCPRecieverRecv:
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

		for {
			p := <-track.Packets
			if bytes.Equal(p.Payload, []byte{0x10, 0x00}) {
				close(awaitRTPRecv)
				return
			}
		}
	})

	vp8Track, err := pcOffer.NewRTCSampleTrack(DefaultPayloadTypeVP8, "video", "pion")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pcOffer.AddTrack(vp8Track); err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			time.Sleep(time.Millisecond * 100)
			vp8Track.Samples <- media.RTCSample{Data: []byte{0x00}, Samples: 1}

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
			if routineErr := pcOffer.SendRTCP(&rtcp.PictureLossIndication{SenderSSRC: vp8Track.Ssrc, MediaSSRC: vp8Track.Ssrc}); routineErr != nil {
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
}
