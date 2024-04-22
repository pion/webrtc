// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// simulcast demonstrates of how to handle incoming track with multiple simulcast rtp streams and show all them back.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/examples/internal/signal"
)

// nolint:gocognit
func main() {
	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer func() {
		if cErr := peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	outputTracks := map[string]*webrtc.TrackLocalStaticRTP{}

	// Create Track that we send video back to browser on
	outputTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video_q", "pion_q")
	if err != nil {
		panic(err)
	}
	outputTracks["q"] = outputTrack

	outputTrack, err = webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video_h", "pion_h")
	if err != nil {
		panic(err)
	}
	outputTracks["h"] = outputTrack

	outputTrack, err = webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video_f", "pion_f")
	if err != nil {
		panic(err)
	}
	outputTracks["f"] = outputTrack

	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		panic(err)
	}

	// Add this newly created track to the PeerConnection to send back video
	if _, err = peerConnection.AddTransceiverFromTrack(outputTracks["q"], webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTransceiverFromTrack(outputTracks["h"], webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTransceiverFromTrack(outputTracks["f"], webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); err != nil {
		panic(err)
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	processRTCP := func(rtpSender *webrtc.RTPSender) {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}
	for _, rtpSender := range peerConnection.GetSenders() {
		go processRTCP(rtpSender)
	}

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	signal.Decode(signal.MustReadStdin(), &offer)

	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) { //nolint: revive
		fmt.Println("Track has started")

		// Start reading from all the streams and sending them to the related output track
		rid := track.RID()
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			for range ticker.C {
				fmt.Printf("Sending pli for stream with rid: %q, ssrc: %d\n", track.RID(), track.SSRC())
				if writeErr := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}}); writeErr != nil {
					fmt.Println(writeErr)
				}
			}
		}()
		for {
			// Read RTP packets being sent to Pion
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}

			if writeErr := outputTracks[rid].WriteRTP(packet); writeErr != nil && !errors.Is(writeErr, io.ErrClosedPipe) {
				panic(writeErr)
			}
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}

		if s == webrtc.PeerConnectionStateClosed {
			// PeerConnection was explicitly closed. This usually happens from a DTLS CloseNotify
			fmt.Println("Peer Connection has gone to closed exiting")
			os.Exit(0)
		}
	})

	// Create an answer
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
	fmt.Println(signal.Encode(*peerConnection.LocalDescription()))

	// Block forever
	select {}
}
