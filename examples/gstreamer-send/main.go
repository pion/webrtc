package main

import (
	"fmt"
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
	if err != nil {
		panic(err)
	}

	fmt.Println("")
	sd, err := base64.StdEncoding.DecodeString(rawSd)
	if err != nil {
		panic(err)
	}

	/* Everything below is the pion-WebRTC API, thanks for using it! */

	// Create a new RTCPeerConnection
	peerConnection, _ := webrtc.New(&webrtc.RTCConfiguration{})

	// Create a audio track
	opusIn, err := peerConnection.AddTrack(webrtc.Opus, 48000)
	if err != nil {
		panic(err)
	}

	// Create a video track
	vp8In, err := peerConnection.AddTrack(webrtc.VP8, 90000)
	if err != nil {
		panic(err)
	}

	// Set the remote SessionDescription
	if err := peerConnection.SetRemoteDescription(string(sd)); err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err := peerConnection.CreateAnswer(); err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	}

	// Get the LocalDescription and take it to base64 so we can paste in browser
	localDescriptionStr := peerConnection.LocalDescription.Marshal()
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(localDescriptionStr)))

	// Start pushing buffers on these tracks
	gst.CreatePipeline(webrtc.Opus, opusIn).Start()
	gst.CreatePipeline(webrtc.VP8, vp8In).Start()
	select {}
}
