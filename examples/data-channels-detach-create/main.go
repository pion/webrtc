package main

import (
	"fmt"
	"time"

	"github.com/pions/datachannel"
	"github.com/pions/webrtc"

	"github.com/pions/webrtc/examples/internal/signal"
)

const messageSize = 15

func main() {
	// Since this behavior diverges from the WebRTC API it has to be
	// enabled using a settings engine. Mixing both detached and the
	// OnMessage DataChannel API is not supported.

	// Create a SettingEngine and enable Detach
	s := webrtc.SettingEngine{}
	s.DetachDataChannels()

	// Create an API object with the engine
	api := webrtc.NewAPI(webrtc.WithSettingEngine(s))

	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection using the API object
	peerConnection, err := api.NewPeerConnection(config)
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
		fmt.Printf("Data channel '%s'-'%d' open.\n", dataChannel.Label, dataChannel.ID)

		// Detach the data channel
		raw, dErr := dataChannel.Detach()
		if dErr != nil {
			panic(dErr)
		}

		// Handle reading from the data channel
		go ReadLoop(raw)

		// Handle writing to the data channel
		go WriteLoop(raw)
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

// ReadLoop shows how to read from the datachannel directly
func ReadLoop(d *datachannel.DataChannel) {
	for {
		buffer := make([]byte, messageSize)
		n, err := d.Read(buffer)
		if err != nil {
			fmt.Println("Datachannel closed; Exit the readloop:", err)
			return
		}

		fmt.Printf("Message from DataChannel '%s': %s\n", d.Label, string(buffer[:n]))
	}
}

// WriteLoop shows how to write to the datachannel directly
func WriteLoop(d *datachannel.DataChannel) {
	for range time.NewTicker(5 * time.Second).C {
		message := signal.RandSeq(messageSize)
		fmt.Printf("Sending %s \n", message)

		_, err := d.Write([]byte(message))
		if err != nil {
			panic(err)
		}
	}
}
