package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/pions/webrtc"
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

	// Create a new RTCPeerConnection, providing ICE servers
	peerConnection, err := webrtc.New(webrtc.RTCConfiguration{
		ICEServers: []webrtc.RTCICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	peerConnection.Ontrack = func(track *webrtc.RTCTrack) {
		fmt.Printf("Got a %s track\n", track.Codec.Name)
	}

	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
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
	select {}
}
