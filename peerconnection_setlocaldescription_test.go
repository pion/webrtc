// +build !js

package webrtc

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/pion/webrtc/v2/pkg/media"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func runOfferingPeer(offerChan chan<- SessionDescription, answerChan <-chan SessionDescription) {
	config := Configuration{}
	peerConnection, err := NewPeerConnection(config)
	check(err)

	track, err := peerConnection.NewTrack(DefaultPayloadTypeOpus, rand.Uint32(), "audio", "pion1")
	check(err)
	_, err = peerConnection.AddTrack(track)
	check(err)

	offer, err := peerConnection.CreateOffer(nil)
	check(err)
	err = peerConnection.SetLocalDescription(offer)
	check(err)
	offerChan <- offer

	answer := <-answerChan
	err = peerConnection.SetRemoteDescription(answer)
	check(err)

	for {
		// send bogus data
		sample := media.Sample{
			Data:    []byte{0x00},
			Samples: 1,
		}
		err = track.WriteSample(sample)
		check(err)
	}
}

func runAnsweringPeer(offerChan <-chan SessionDescription, answerChan chan<- SessionDescription, resultChan chan<- error) {
	s := SettingEngine{
		LoggerFactory: testCatchAllLoggerFactory{
			callback: func(msg string) {
				if strings.Contains(msg, "SetLocalDescription not called") {
					resultChan <- nil
				}
			},
		},
	}
	api := NewAPI(WithSettingEngine(s))
	api.mediaEngine.RegisterDefaultCodecs()

	peerConnection, err := api.NewPeerConnection(Configuration{})
	check(err)

	_, err = peerConnection.AddTransceiverFromKind(RTPCodecTypeAudio)
	check(err)

	peerConnection.OnTrack(func(track *Track, receiver *RTPReceiver) {
		buf := make([]byte, 1400)
		_, err = track.Read(buf)
		check(err)
		resultChan <- errors.New("Data erroneously received")
	})

	offer := <-offerChan
	err = peerConnection.SetRemoteDescription(offer)
	check(err)

	answer, err := peerConnection.CreateAnswer(nil)
	check(err)
	answerChan <- answer
}

func TestNoPanicIfSetLocalDescriptionNotCalledByAnsweringPeer(t *testing.T) {
	offerChan := make(chan SessionDescription)
	answerChan := make(chan SessionDescription)
	resultChan := make(chan error)

	go runOfferingPeer(offerChan, answerChan)
	go runAnsweringPeer(offerChan, answerChan, resultChan)

	// wait for either:
	// - the expected logging (success!)
	// - the read to succeed (which is actually a bad thing)
	// - or a timeout (also bad)
	select {
	case err := <-resultChan:
		if err != nil {
			t.Fatal(err.Error())
		}
	case <-time.After(140 * time.Second):
		t.Fatalf("Timed out waiting for expected logging")
	}
}
