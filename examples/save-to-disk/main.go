package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/pions/webrtc"
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
	peerConnection, err := webrtc.New(&webrtc.RTCConfiguration{
		ICEServers: []webrtc.RTCICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts, this handler saves buffers to disk as
	// an ivf file, since we could have multiple video tracks we provide a counter.
	// In your application this is where you would handle/process video
	peerConnection.Ontrack = func(mediaType webrtc.TrackType, packets <-chan *rtp.Packet) {
		if mediaType == webrtc.VP8 {
			fmt.Println("Got VP8 track, saving to disk as output.ivf")
			i, err := newIVFWriter("output.ivf")
			if err != nil {
				panic(err)
			}
			for {
				i.addPacket(<-packets)
			}
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
