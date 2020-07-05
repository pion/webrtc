package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/examples/internal/signal"
)

func main() {
	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	signal.Decode(signal.MustReadStdin(), &offer)

	// We make our own mediaEngine so we can place the sender's codecs in it. Since we are echoing their RTP packet
	// back to them we are actually codec agnostic - we can accept all their codecs. This also ensures that we use the
	// dynamic media type from the sender in our answer.
	mediaEngine := webrtc.MediaEngine{}

	// Add codecs to the mediaEngine. Note that even though we are only going to echo back the sender's video we also
	// add audio codecs. This is because createAnswer will create an audioTransceiver and associated SDP and we currently
	// cannot tell it not to. The audio SDP must match the sender's codecs too...
	err := mediaEngine.PopulateFromSDP(offer)
	if err != nil {
		panic(err)
	}

	videoCodecs := mediaEngine.GetCodecsByKind(webrtc.RTPCodecTypeVideo)
	if len(videoCodecs) == 0 {
		panic("Offer contained no video codecs")
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))

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

	// Create Track that we send video back to browser on
	outputTrack, err := peerConnection.NewTrack(videoCodecs[0].PayloadType, rand.Uint32(), "video", "pion")
	if err != nil {
		panic(err)
	}

	// Add this newly created track to the PeerConnection
	if _, err = peerConnection.AddTrack(outputTrack); err != nil {
		panic(err)
	}

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts, this handler copies inbound RTP packets,
	// replaces the SSRC and sends them back
	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This is a temporary fix until we implement incoming RTCP events, then we would push a PLI only when a viewer requests it
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()}})
				if errSend != nil {
					fmt.Println(errSend)
				}
			}
		}()

		fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), track.Codec().Name)
		for {
			// Read RTP packets being sent to Pion
			rtp, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}

			// Replace the SSRC with the SSRC of the outbound track.
			// The only change we are making replacing the SSRC, the RTP packets are unchanged otherwise
			rtp.SSRC = outputTrack.SSRC()

			if writeErr := outputTrack.WriteRTP(rtp); writeErr != nil {
				panic(writeErr)
			}
		}
	})
	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
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
	fmt.Println(signal.Encode(*peerConnection.LocalDescription()))

	// Block forever
	select {}
}
