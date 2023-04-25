// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// bandwidth-estimation-from-disk demonstrates how to use Pion's Bandwidth Estimation APIs.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/interceptor/pkg/gcc"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/examples/internal/signal"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/ivfreader"
)

const (
	lowFile    = "low.ivf"
	lowBitrate = 300_000

	medFile    = "med.ivf"
	medBitrate = 1_000_000

	highFile    = "high.ivf"
	highBitrate = 2_500_000

	ivfHeaderSize = 32
)

// nolint: gocognit
func main() {
	qualityLevels := []struct {
		fileName string
		bitrate  int
	}{
		{lowFile, lowBitrate},
		{medFile, medBitrate},
		{highFile, highBitrate},
	}
	currentQuality := 0

	for _, level := range qualityLevels {
		_, err := os.Stat(level.fileName)
		if os.IsNotExist(err) {
			panic(fmt.Sprintf("File %s was not found", level.fileName))
		}
	}

	i := &interceptor.Registry{}
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	// Create a Congestion Controller. This analyzes inbound and outbound data and provides
	// suggestions on how much we should be sending.
	//
	// Passing `nil` means we use the default Estimation Algorithm which is Google Congestion Control.
	// You can use the other ones that Pion provides, or write your own!
	congestionController, err := cc.NewInterceptor(func() (cc.BandwidthEstimator, error) {
		return gcc.NewSendSideBWE(gcc.SendSideBWEInitialBitrate(lowBitrate))
	})
	if err != nil {
		panic(err)
	}

	estimatorChan := make(chan cc.BandwidthEstimator, 1)
	congestionController.OnNewPeerConnection(func(id string, estimator cc.BandwidthEstimator) {
		estimatorChan <- estimator
	})

	i.Add(congestionController)
	if err = webrtc.ConfigureTWCCHeaderExtensionSender(m, i); err != nil {
		panic(err)
	}

	if err = webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		panic(err)
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewAPI(webrtc.WithInterceptorRegistry(i), webrtc.WithMediaEngine(m)).NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	defer func() {
		if cErr := peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	// Wait until our Bandwidth Estimator has been created
	estimator := <-estimatorChan

	// Create a video track
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion")
	if err != nil {
		panic(err)
	}

	rtpSender, err := peerConnection.AddTrack(videoTrack)
	if err != nil {
		panic(err)
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())
	})

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	signal.Decode(signal.MustReadStdin(), &offer)

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(signal.Encode(*peerConnection.LocalDescription()))

	// Open a IVF file and start reading using our IVFReader
	file, err := os.Open(qualityLevels[currentQuality].fileName)
	if err != nil {
		panic(err)
	}

	ivf, header, err := ivfreader.NewWith(file)
	if err != nil {
		panic(err)
	}

	// Send our video file frame at a time. Pace our sending so we send it at the same speed it should be played back as.
	// This isn't required since the video is timestamped, but we will such much higher loss if we send all at once.
	//
	// It is important to use a time.Ticker instead of time.Sleep because
	// * avoids accumulating skew, just calling time.Sleep didn't compensate for the time spent parsing the data
	// * works around latency issues with Sleep (see https://github.com/golang/go/issues/44343)
	ticker := time.NewTicker(time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000))
	frame := []byte{}
	frameHeader := &ivfreader.IVFFrameHeader{}
	currentTimestamp := uint64(0)

	switchQualityLevel := func(newQualityLevel int) {
		fmt.Printf("Switching from %s to %s \n", qualityLevels[currentQuality].fileName, qualityLevels[newQualityLevel].fileName)
		currentQuality = newQualityLevel
		ivf.ResetReader(setReaderFile(qualityLevels[currentQuality].fileName))
		for {
			if frame, frameHeader, err = ivf.ParseNextFrame(); err != nil {
				break
			} else if frameHeader.Timestamp >= currentTimestamp && frame[0]&0x1 == 0 {
				break
			}
		}
	}

	for ; true; <-ticker.C {
		targetBitrate := estimator.GetTargetBitrate()
		switch {
		// If current quality level is below target bitrate drop to level below
		case currentQuality != 0 && targetBitrate < qualityLevels[currentQuality].bitrate:
			switchQualityLevel(currentQuality - 1)

			// If next quality level is above target bitrate move to next level
		case len(qualityLevels) > (currentQuality+1) && targetBitrate > qualityLevels[currentQuality+1].bitrate:
			switchQualityLevel(currentQuality + 1)

		// Adjust outbound bandwidth for probing
		default:
			frame, _, err = ivf.ParseNextFrame()
		}

		switch {
		// If we have reached the end of the file start again
		case errors.Is(err, io.EOF):
			ivf.ResetReader(setReaderFile(qualityLevels[currentQuality].fileName))

		// No error write the video frame
		case err == nil:
			currentTimestamp = frameHeader.Timestamp
			if err = videoTrack.WriteSample(media.Sample{Data: frame, Duration: time.Second}); err != nil {
				panic(err)
			}
		// Error besides io.EOF that we dont know how to handle
		default:
			panic(err)
		}
	}
}

func setReaderFile(filename string) func(_ int64) io.Reader {
	return func(_ int64) io.Reader {
		file, err := os.Open(filename) // nolint
		if err != nil {
			panic(err)
		}
		if _, err = file.Seek(ivfHeaderSize, io.SeekStart); err != nil {
			panic(err)
		}
		return file
	}
}
