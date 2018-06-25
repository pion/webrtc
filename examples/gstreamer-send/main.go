package main

import (
	"fmt"
	"os"

	"bufio"
	"encoding/base64"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/gstreamer-send/gst"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Browser base64 Session Description: ")
	rawSd, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	fmt.Println("\nGolang base64 Session Description: ")

	sd, err := base64.StdEncoding.DecodeString(rawSd)
	if err != nil {
		panic(err)
	}

	/* Everything below is the pion-WebRTC API, thanks for using it! */

	// Create a new RTCPeerConnection
	peerConnection := &webrtc.RTCPeerConnection{}

	// Create a video track, and start pushing buffers
	in, err := peerConnection.AddTrack(webrtc.VP8)
	if err != nil {
		panic(err)
	}

	// Set the remote SessionDescription
	if err := peerConnection.SetRemoteDescription(string(sd)); err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err := peerConnection.CreateOffer(); err != nil {
		panic(err)
	}

	// Get the LocalDescription and take it to base64 so we can paste in browser
	localDescriptionStr := peerConnection.LocalDescription.Marshal()
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(localDescriptionStr)))

	gst.CreatePipeline(in).Start()
	select {}
}
