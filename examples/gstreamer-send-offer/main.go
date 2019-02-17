package main

import (
	"fmt"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
	gst "github.com/pions/webrtc/examples/util/gstreamer-src"
	"github.com/pions/webrtc/pkg/ice"
)

func main() {
	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Setup the codecs you want to use.
	// We'll use the default ones but you can also define your own
	webrtc.RegisterDefaultCodecs()

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
	util.Check(err)

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Create a audio track
	opusTrack, err := peerConnection.NewSampleTrack(webrtc.DefaultPayloadTypeOpus, "audio", "pion1")
	util.Check(err)
	_, err = peerConnection.AddTrack(opusTrack)
	util.Check(err)

	// Create a video track
	vp8Track, err := peerConnection.NewSampleTrack(webrtc.DefaultPayloadTypeVP8, "video", "pion2")
	util.Check(err)
	_, err = peerConnection.AddTrack(vp8Track)
	util.Check(err)

	// Create an offer to send to the browser
	offer, err := peerConnection.CreateOffer(nil)
	util.Check(err)

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(offer)
	util.Check(err)

	// Output the offer in base64 so we can paste it in browser
	fmt.Println(util.Encode(offer))

	// Wait for the answer to be pasted
	answer := webrtc.SessionDescription{}
	util.Decode(util.MustReadStdin(), &answer)

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(answer)
	util.Check(err)

	// Start pushing buffers on these tracks
	gst.CreatePipeline(webrtc.Opus, opusTrack.Samples, "audiotestsrc").Start()
	gst.CreatePipeline(webrtc.VP8, vp8Track.Samples, "videotestsrc").Start()

	// Block forever
	select {}
}
