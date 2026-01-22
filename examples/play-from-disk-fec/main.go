// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// play-from-disk-fec demonstrates how to use forward error correction (FlexFEC-03)
// while sending video to your Chrome-based browser from files saved to disk.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
)

const (
	videoFileName  = "output.ivf"
	answerFileName = "answer.txt"
)

func main() { //nolint:gocognit,cyclop,gocyclo,maintidx
	// Assert that we have a video file
	_, err := os.Stat(videoFileName)

	if os.IsNotExist(err) {
		panic("Could not find `" + videoFileName + "`")
	}

	// Create mediaEngine with default codecs
	mediaEngine := &webrtc.MediaEngine{}
	if err = mediaEngine.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	// Create interceptorRegistry with default interceptots
	interceptorRegistry := &interceptor.Registry{}

	interceptorRegistry.Add(packetDropInterceptorFactory{})

	// Configure flexfec-03
	if err = webrtc.ConfigureFlexFEC03(49, mediaEngine, interceptorRegistry); err != nil {
		panic(err)
	}

	if err = webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		panic(err)
	}

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(interceptorRegistry),
	)

	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
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

	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())

	file, openErr := os.Open(videoFileName)
	if openErr != nil {
		panic(openErr)
	}

	_, header, openErr := ivfreader.NewWith(file)
	if openErr != nil {
		panic(openErr)
	}

	// Determine video codec
	var trackCodec string
	switch header.FourCC {
	case "AV01":
		trackCodec = webrtc.MimeTypeAV1
	case "VP90":
		trackCodec = webrtc.MimeTypeVP9
	case "VP80":
		trackCodec = webrtc.MimeTypeVP8
	default:
		panic(fmt.Sprintf("Unable to handle FourCC %s", header.FourCC))
	}

	// Create a video track
	videoTrack, videoTrackErr := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: trackCodec}, "video", "pion",
	)
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

	rtpSender, videoTrackErr := peerConnection.AddTrack(videoTrack)
	if videoTrackErr != nil {
		panic(videoTrackErr)
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

	go func() {
		// Open a IVF file and start reading using our IVFReader
		file, ivfErr := os.Open(videoFileName)
		if ivfErr != nil {
			panic(ivfErr)
		}

		ivf, header, ivfErr := ivfreader.NewWith(file)
		if ivfErr != nil {
			panic(ivfErr)
		}

		// Wait for connection established
		<-iceConnectedCtx.Done()

		// Send our video file frame at a time. Pace our sending so we send it at the same speed it should be played back as.
		// This isn't required since the video is timestamped, but we will such much higher loss if we send all at once.
		//
		// It is important to use a time.Ticker instead of time.Sleep because
		// * avoids accumulating skew, just calling time.Sleep didn't compensate for the time spent parsing the data
		// * works around latency issues with Sleep (see https://github.com/golang/go/issues/44343)
		ticker := time.NewTicker(
			time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000),
		)
		defer ticker.Stop()
		for ; true; <-ticker.C {
			frame, _, ivfErr := ivf.ParseNextFrame()
			if errors.Is(ivfErr, io.EOF) {
				fmt.Printf("All video frames parsed and sent")
				os.Exit(0)
			}

			if ivfErr != nil {
				panic(ivfErr)
			}

			if ivfErr = videoTrack.WriteSample(media.Sample{Data: frame, Duration: time.Second}); ivfErr != nil {
				panic(ivfErr)
			}
		}
	}()

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", state.String())

		if state == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure.
			// It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}

		if state == webrtc.PeerConnectionStateClosed {
			// PeerConnection was explicitly closed. This usually happens from a DTLS CloseNotify
			fmt.Println("Peer Connection has gone to closed exiting")
			os.Exit(0)
		}
	})

	// Create offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Output the offer in base64 so we can paste it in browser
	fmt.Println(encode(peerConnection.LocalDescription()))

	// Wait for user to save the answer and press enter
	fmt.Printf("Save the browser's answer to '%s' and press Enter to continue...\n", answerFileName)
	_, err = bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		panic(err)
	}

	// Read the answer from file
	answerData, readErr := os.ReadFile(answerFileName)
	if readErr != nil {
		panic(readErr)
	}

	answerStr := strings.TrimSpace(string(answerData))
	if len(answerStr) == 0 {
		panic("Answer file is empty")
	}

	answer := webrtc.SessionDescription{}
	decode(answerStr, &answer)

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(answer); err != nil {
		panic(err)
	}

	fmt.Println("Answer received and set successfully!")

	// Block forever
	select {}
}

// JSON encode + base64 a SessionDescription.
func encode(obj *webrtc.SessionDescription) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode a base64 and unmarshal JSON into a SessionDescription.
func decode(in string, obj *webrtc.SessionDescription) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}

	if err = json.Unmarshal(b, obj); err != nil {
		panic(err)
	}
}

// Factory for creating the interceptor.
type packetDropInterceptorFactory struct{}

func (f packetDropInterceptorFactory) NewInterceptor(_ string) (interceptor.Interceptor, error) {
	return &dropFilter{}, nil
}

// dropFilter drops outgoing video packets based on sequence number.
type dropFilter struct {
	interceptor.NoOp
	mu                  sync.Mutex
	mediaPacketsTotal   int
	fecPacketsTotal     int
	droppedPacketsTotal int
}

func (i *dropFilter) BindLocalStream(info *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
	if !strings.HasPrefix(strings.ToLower(info.MimeType), "video/") {
		return writer
	}

	return interceptor.RTPWriterFunc(func(header *rtp.Header, payload []byte, attrs interceptor.Attributes) (int, error) {
		i.mu.Lock()
		defer i.mu.Unlock()

		// Check if this is a FEC packet
		if header.SSRC == info.SSRCForwardErrorCorrection {
			i.fecPacketsTotal++

			return writer.Write(header, payload, attrs)
		}

		// Log stats periodically
		if i.mediaPacketsTotal%100 == 0 {
			dropRatio := float64(i.droppedPacketsTotal) / float64(i.mediaPacketsTotal)
			fmt.Printf("Stats: Media: %d, FEC: %d, Dropped: %d, Drop ratio: %.4f%%\n",
				i.mediaPacketsTotal, i.fecPacketsTotal, i.droppedPacketsTotal, dropRatio*100)
		}

		// Count all media packets
		i.mediaPacketsTotal++

		// 40% loss
		if i.mediaPacketsTotal%5 <= 1 {
			i.droppedPacketsTotal++

			return len(payload), nil // Pretend we wrote the packet but actually drop it
		}

		return writer.Write(header, payload, attrs)
	})
}
