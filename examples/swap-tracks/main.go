// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// swap-tracks demonstrates how to swap multiple incoming tracks on a single outgoing track.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/examples/internal/signal"
)

func main() { // nolint:gocognit
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

	// Create Track that we send video back to browser on
	outputTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion")
	if err != nil {
		panic(err)
	}

	// Add this newly created track to the PeerConnection
	rtpSender, err := peerConnection.AddTrack(outputTrack)
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

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	signal.Decode(signal.MustReadStdin(), &offer)

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Which track is currently being handled
	currTrack := 0
	// The total number of tracks
	trackCount := 0
	// The channel of packets with a bit of buffer
	packets := make(chan *rtp.Packet, 60)

	// Set a handler for when a new remote track starts
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), track.Codec().MimeType)
		trackNum := trackCount
		trackCount++
		// The last timestamp so that we can change the packet to only be the delta
		var lastTimestamp uint32

		// Whether this track is the one currently sending to the channel (on change
		// of this we send a PLI to have the entire picture updated)
		var isCurrTrack bool
		for {
			// Read RTP packets being sent to Pion
			rtp, _, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}

			// Change the timestamp to only be the delta
			oldTimestamp := rtp.Timestamp
			if lastTimestamp == 0 {
				rtp.Timestamp = 0
			} else {
				rtp.Timestamp -= lastTimestamp
			}
			lastTimestamp = oldTimestamp

			// Check if this is the current track
			if currTrack == trackNum {
				// If just switched to this track, send PLI to get picture refresh
				if !isCurrTrack {
					isCurrTrack = true
					if writeErr := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}}); writeErr != nil {
						fmt.Println(writeErr)
					}
				}
				packets <- rtp
			} else {
				isCurrTrack = false
			}
		}
	})

	ctx, done := context.WithCancel(context.Background())

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			done()
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

	fmt.Println(signal.Encode(*peerConnection.LocalDescription()))

	// Asynchronously take all packets in the channel and write them out to our
	// track
	go func() {
		var currTimestamp uint32
		for i := uint16(0); ; i++ {
			packet := <-packets
			// Timestamp on the packet is really a diff, so add it to current
			currTimestamp += packet.Timestamp
			packet.Timestamp = currTimestamp
			// Keep an increasing sequence number
			packet.SequenceNumber = i
			// Write out the packet, ignoring closed pipe if nobody is listening
			if err := outputTrack.WriteRTP(packet); err != nil {
				if errors.Is(err, io.ErrClosedPipe) {
					// The peerConnection has been closed.
					return
				}

				panic(err)
			}
		}
	}()

	// Wait for connection, then rotate the track every 5s
	fmt.Printf("Waiting for connection\n")
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// We haven't gotten any tracks yet
		if trackCount == 0 {
			continue
		}

		fmt.Printf("Waiting 5 seconds then changing...\n")
		time.Sleep(5 * time.Second)
		if currTrack == trackCount-1 {
			currTrack = 0
		} else {
			currTrack++
		}
		fmt.Printf("Switched to track #%v\n", currTrack+1)
	}
}
