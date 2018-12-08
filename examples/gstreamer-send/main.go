package main

import (
	"fmt"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/gstreamer-send/gst"
	"github.com/pions/webrtc/examples/util"
	"github.com/pions/webrtc/pkg/ice"
)

func main() {
	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Setup the codecs you want to use.
	// We'll use the default ones but you can also define your own
	webrtc.RegisterDefaultCodecs()

	// Prepare the configuration
	config := webrtc.RTCConfiguration{
		IceServers: []webrtc.RTCIceServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.New(config)
	util.Check(err)

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Create a audio track
	opusTrack, err := peerConnection.NewRTCSampleTrack(webrtc.DefaultPayloadTypeOpus, "audio", "pion1")
	util.Check(err)
	_, err = peerConnection.AddTrack(opusTrack)
	util.Check(err)

	// Create a video track
	vp8Track, err := peerConnection.NewRTCSampleTrack(webrtc.DefaultPayloadTypeVP8, "video", "pion2")
	util.Check(err)
	_, err = peerConnection.AddTrack(vp8Track)
	util.Check(err)

	// Wait for the offer to be pasted
	offer := util.Decode(util.MustReadStdin())

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	util.Check(err)

	// Sets the LocalDescription, and starts our UDP listeners
	answer, err := peerConnection.CreateAnswer(nil)
	util.Check(err)

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(util.Encode(answer))

	// Start pushing buffers on these tracks
	gst.CreatePipeline(webrtc.Opus, opusTrack.Samples).Start()
	gst.CreatePipeline(webrtc.VP8, vp8Track.Samples).Start()

	// Block forever
	select {}
}
