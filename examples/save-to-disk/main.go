package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/rtp"
)

var trackCount uint64

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
	// an ivf file, since we could have multiple video tracks we provide a counter.
	// In your application this is where you would handle/process video
	peerConnection.Ontrack = func(mediaType webrtc.MediaType, packets chan *rtp.Packet) {
		go func() {
			track := atomic.AddUint64(&trackCount, 1)
			fmt.Printf("Track %d has started \n", track)

			i, err := NewIVFWriter(fmt.Sprintf("output-%d.ivf", track))
			if err != nil {
				panic(err)
			}
			for {
				i.AddPacket(<-packets)
			}
		}()
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
