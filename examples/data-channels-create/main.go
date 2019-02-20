package main

import (
	"fmt"
	"time"

	"github.com/pions/webrtc"
	sugar "github.com/pions/webrtc/pkg/datachannel"

	"github.com/pions/webrtc/examples/internal/signal"
)

func main() {
	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Create a datachannel with label 'data'
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Register channel opening handling
	dataChannel.OnOpen(func() {
		fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", dataChannel.Label, dataChannel.ID)

		for range time.NewTicker(5 * time.Second).C {
			message := signal.RandSeq(15)
			fmt.Printf("Sending %s \n", message)

			err := dataChannel.Send(sugar.PayloadString{Data: []byte(message)})
			if err != nil {
				panic(err)
			}
		}
	})

	// Register the OnMessage to handle incoming messages
	dataChannel.OnMessage(func(payload sugar.Payload) {
		switch p := payload.(type) {
		case *sugar.PayloadString:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '%s'\n", p.PayloadType().String(), dataChannel.Label, string(p.Data))
		case *sugar.PayloadBinary:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '% 02x'\n", p.PayloadType().String(), dataChannel.Label, p.Data)
		default:
			fmt.Printf("Message '%s' from DataChannel '%s' no payload \n", p.PayloadType().String(), dataChannel.Label)
		}
	})

	// Create an offer to send to the browser
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		panic(err)
	}

	// Output the offer in base64 so we can paste it in browser
	fmt.Println(signal.Encode(offer))

	// Wait for the answer to be pasted
	answer := webrtc.SessionDescription{}
	signal.Decode(signal.MustReadStdin(), &answer)

	// Apply the answer as the remote description
	err = peerConnection.SetRemoteDescription(answer)
	if err != nil {
		panic(err)
	}

	// Block forever
	select {}
}
