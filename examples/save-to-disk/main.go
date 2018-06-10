package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/pions/webrtc"
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

	// Set a handler for when a new remote track starts, this handler saves buffers to disk as
	// an ivf file of the users choosing
	peerConnection.Ontrack = func(mediaType webrtc.MediaType, buffers <-chan []byte) {
		fmt.Println("We track was discovered")
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
	select {}
}
