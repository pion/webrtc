// +build !js

package webrtc

import (
	"bufio"
	"errors"
	"io"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/pion/logging"
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
	config := Configuration{}
	peerConnection, err := NewPeerConnection(config)
	check(err)

	initLogWatcher(peerConnection, resultChan)

	_, err = peerConnection.AddTransceiver(RTPCodecTypeAudio)
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

func initLogWatcher(peerConnection *PeerConnection, resultChan chan<- error) {
	expectedLogging := "SetLocalDescription not called"

	r, w := io.Pipe()

	// replace the existing logger with one we can slurp from
	loggerFactory := &logging.DefaultLoggerFactory{
		Writer:          w,
		DefaultLogLevel: logging.LogLevelWarn,
	}
	peerConnection.log = loggerFactory.NewLogger("pc")

	scanner := bufio.NewScanner(r)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, expectedLogging) {
				// we found what we were looking for
				resultChan <- nil
				break
			}
		}
		check(scanner.Err())
	}()
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
