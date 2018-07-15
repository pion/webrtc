package main

import (
	"fmt"
	"io"
	"os"

	"bufio"
	"encoding/base64"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/gstreamer-send/gst"
	"github.com/pions/webrtc/pkg/ice"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	rawSd, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		panic(err)
	}

	fmt.Println("")
	sd, err := base64.StdEncoding.DecodeString(rawSd)
	if err != nil {
		panic(err)
	}

	/* Everything below is the pion-WebRTC API, thanks for using it! */

	// Setup the codecs you want to use.
	// We'll use the default ones but you can also define your own
	webrtc.RegisterDefaultCodecs()

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.New(webrtc.RTCConfiguration{})
	if err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	}

	// Create a audio track
	opusTrack, err := peerConnection.NewRTCTrack(webrtc.PayloadTypeOpus, "audio", "pions1")
	if err != nil {
		panic(err)
	}
	_, err = peerConnection.AddTrack(opusTrack)
	if err != nil {
		panic(err)
	}

	// Create a video track
	vp8Track, err := peerConnection.NewRTCTrack(webrtc.PayloadTypeVP8, "video", "pions2")
	if err != nil {
		panic(err)
	}
	_, err = peerConnection.AddTrack(vp8Track)
	if err != nil {
		panic(err)
	}

	// Set the remote SessionDescription
	offer := webrtc.RTCSessionDescription{
		Typ: webrtc.RTCSdpTypeOffer,
		Sdp: string(sd),
	}
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Get the LocalDescription and take it to base64 so we can paste in browser
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(answer.Sdp)))

	// Start pushing buffers on these tracks
	gst.CreatePipeline(webrtc.Opus, opusTrack.Samples).Start()
	gst.CreatePipeline(webrtc.VP8, vp8Track.Samples).Start()
	select {}
}
