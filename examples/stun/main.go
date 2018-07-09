package main

import (
	"encoding/base64"
	"fmt"

	"github.com/pions/webrtc"
)

func main() {
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

	if err := peerConnection.CreateAnswer(); err != nil {
		panic(err)
	}

	localDescriptionStr := peerConnection.LocalDescription.Marshal()
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(localDescriptionStr)))
	select {}
}
