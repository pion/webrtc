// +build !js

package main

import (
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

	// Create Track that we send video back to browser on
	outputTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")
	if err != nil {
		panic(err)
	}

	// Add this newly created track to the PeerConnection
	if _, err = peerConnection.AddTrack(outputTrack); err != nil {
		panic(err)
	}

	// In addition to the implicit transceiver added by the track, we add two more
	// for the other tracks
	_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RtpTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
	if err != nil {
		panic(err)
	}
	_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RtpTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
	if err != nil {
		panic(err)
	}

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
			rtp, readErr := track.ReadRTP()
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
	// Set the handler for ICE connection state and update chan if connected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
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
	fmt.Printf("Paste below base64 in browser:\n%v\n", signal.Encode(*peerConnection.LocalDescription()))

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
			if err := outputTrack.WriteRTP(packet); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				panic(err)
			}
		}
	}()

	// Wait for connection, then rotate the track every 5s
	fmt.Printf("Waiting for connection\n")
	for {
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
