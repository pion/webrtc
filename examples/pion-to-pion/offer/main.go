package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
)

func main() {
	addr := flag.String("address", ":50000", "Address that the HTTP server is hosted on.")
	flag.Parse()

	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.RTCConfiguration{
		IceServers: []webrtc.RTCIceServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.New(config)
	util.Check(err)

	// Create a datachannel with label 'data'
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	util.Check(err)

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState ice.ConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Register channel opening handling
	dataChannel.OnOpen(func() {
		fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", dataChannel.Label, dataChannel.ID)

		for range time.NewTicker(5 * time.Second).C {
			message := util.RandSeq(15)
			fmt.Printf("Sending %s \n", message)

			err := dataChannel.Send(datachannel.PayloadString{Data: []byte(message)})
			util.Check(err)
		}
	})

	// Register the OnMessage to handle incoming messages
	dataChannel.OnMessage(func(payload datachannel.Payload) {
		switch p := payload.(type) {
		case *datachannel.PayloadString:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '%s'\n", p.PayloadType().String(), dataChannel.Label, string(p.Data))
		case *datachannel.PayloadBinary:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '% 02x'\n", p.PayloadType().String(), dataChannel.Label, p.Data)
		default:
			fmt.Printf("Message '%s' from DataChannel '%s' no payload \n", p.PayloadType().String(), dataChannel.Label)
		}
	})

	// Create an offer to send to the browser
	offer, err := peerConnection.CreateOffer(nil)
	util.Check(err)

	// Exchange the offer for the answer
	answer := mustSignalViaHTTP(offer, *addr)

	// Apply the answer as the remote description
	err = peerConnection.SetRemoteDescription(answer)
	util.Check(err)

	// Block forever
	select {}
}

// mustSignalViaHTTP exchange the SDP offer and answer using an HTTP Post request.
func mustSignalViaHTTP(offer webrtc.RTCSessionDescription, address string) webrtc.RTCSessionDescription {
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(offer)
	util.Check(err)

	resp, err := http.Post("http://"+address, "application/json; charset=utf-8", b)
	util.Check(err)
	defer resp.Body.Close()

	var answer webrtc.RTCSessionDescription
	err = json.NewDecoder(resp.Body).Decode(&answer)
	util.Check(err)

	return answer
}
