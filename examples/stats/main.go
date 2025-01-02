// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// stats demonstrates how to use the webrtc-stats implementation provided by Pion WebRTC.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/webrtc/v4"
)

// How ofter to print WebRTC stats.
const statsInterval = time.Second * 5

// nolint:gocognit,cyclop
func main() {
	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

	// Create a MediaEngine object to configure the supported codec
	mediaEngine := &webrtc.MediaEngine{}

	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	// Create a InterceptorRegistry. This is the user configurable RTP/RTCP Pipeline.
	// This provides NACKs, RTCP Reports and other features. If you use `webrtc.NewPeerConnection`
	// this is enabled by default. If you are manually managing You MUST create a InterceptorRegistry
	// for each PeerConnection.
	interceptorRegistry := &interceptor.Registry{}

	statsInterceptorFactory, err := stats.NewInterceptor()
	if err != nil {
		panic(err)
	}

	var statsGetter stats.Getter
	statsInterceptorFactory.OnNewPeerConnection(func(_ string, g stats.Getter) {
		statsGetter = g
	})
	interceptorRegistry.Add(statsInterceptorFactory)

	// Use the default set of Interceptors
	if err = webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		panic(err)
	}

	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine), webrtc.WithInterceptorRegistry(interceptorRegistry))

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Allow us to receive 1 audio track, and 1 video track
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	} else if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts. We read the incoming packets, but then
	// immediately discard them
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) { //nolint: revive
		fmt.Printf("New incoming track with codec: %s\n", track.Codec().MimeType)

		go func() {
			// Print the stats for this individual track
			for {
				stats := statsGetter.Get(uint32(track.SSRC()))

				fmt.Printf("Stats for: %s\n", track.Codec().MimeType)
				fmt.Println(stats.InboundRTPStreamStats)

				time.Sleep(statsInterval)
			}
		}()

		rtpBuff := make([]byte, 1500)
		for {
			_, _, readErr := track.Read(rtpBuff)
			if readErr != nil {
				panic(readErr)
			}
		}
	})

	var iceConnectionState atomic.Value
	iceConnectionState.Store(webrtc.ICEConnectionStateNew)

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		iceConnectionState.Store(connectionState)
	})

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	decode(readUntilNewline(), &offer)

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
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
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(encode(peerConnection.LocalDescription()))

	for {
		time.Sleep(statsInterval)

		// Stats are only printed after completed to make Copy/Pasting easier
		if iceConnectionState.Load() == webrtc.ICEConnectionStateChecking {
			continue
		}

		// Only print the remote IPs seen
		for _, s := range peerConnection.GetStats() {
			switch stat := s.(type) {
			case webrtc.ICECandidateStats:
				if stat.Type == webrtc.StatsTypeRemoteCandidate {
					fmt.Printf("%s IP(%s) Port(%d)\n", stat.Type, stat.IP, stat.Port)
				}
			default:
			}
		}
	}
}

// Read from stdin until we get a newline.
func readUntilNewline() (in string) {
	var err error

	r := bufio.NewReader(os.Stdin)
	for {
		in, err = r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			panic(err)
		}

		if in = strings.TrimSpace(in); len(in) > 0 {
			break
		}
	}

	fmt.Println("")

	return
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
