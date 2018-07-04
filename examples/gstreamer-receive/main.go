package main

import (
	"fmt"
	"os"

	"bufio"
	"encoding/base64"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/gstreamer-receive/gst"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtp"
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
	peerConnection := &webrtc.RTCPeerConnection{}

	// Set a handler for when a new remote track starts, this handler creates a gstreamer pipeline
	// for the given codec
	peerConnection.Ontrack = func(mediaType webrtc.TrackType, packets <-chan *rtp.Packet) {
		fmt.Printf("Track has started, of type %s \n", mediaType.String())
		pipeline := gst.CreatePipeline(mediaType)
		pipeline.Start()
		for {
			p := <-packets
			pipeline.Push(p.Raw)
		}
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	}

	// Set the remote SessionDescription
	if err := peerConnection.SetRemoteDescription(string(sd)); err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err := peerConnection.CreateAnswer(); err != nil {
		panic(err)
	}

	// Get the LocalDescription and take it to base64 so we can paste in browser
	localDescriptionStr := peerConnection.LocalDescription.Marshal()
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(localDescriptionStr)))
	select {}
}
