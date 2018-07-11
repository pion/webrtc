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

	peerConnection.Ontrack = func(mediaType webrtc.TrackType, packets <-chan *rtp.Packet) {
		fmt.Printf("Got a %s track\n", mediaType)
	}

	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	}

	if err := peerConnection.SetRemoteDescription(string(sd)); err != nil {
		panic(err)
	}

	if err := peerConnection.CreateAnswer(); err != nil {
		panic(err)
	}

	localDescriptionStr := peerConnection.LocalDescription.Marshal()
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(localDescriptionStr)))
	select {}
}
