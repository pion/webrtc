package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
)

func main() {
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
	check(err)

	// Create a datachannel with label 'data'
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	check(err)

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())

		// TODO: find the correct place for this
		if connectionState == ice.ConnectionStateConnected {
			time.AfterFunc(3*time.Second, func() {
				fmt.Println("sending openchannel")
				err := dataChannel.SendOpenChannelMessage()
				if err != nil {
					fmt.Println("faild to send openchannel", err)
				}
			})
		}
	}

	// Register the Onmessage to handle incoming messages
	dataChannel.Lock()
	dataChannel.Onmessage = func(payload datachannel.Payload) {
		switch p := payload.(type) {
		case *datachannel.PayloadString:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '%s'\n", p.PayloadType().String(), dataChannel.Label, string(p.Data))
		case *datachannel.PayloadBinary:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '% 02x'\n", p.PayloadType().String(), dataChannel.Label, p.Data)
		default:
			fmt.Printf("Message '%s' from DataChannel '%s' no payload \n", p.PayloadType().String(), dataChannel.Label)
		}
	}
	dataChannel.Unlock()

	// Create an offer to send to the browser
	offer, err := peerConnection.CreateOffer(nil)
	check(err)

	// Output the offer in base64 so we can paste it in browser
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(offer.Sdp)))

	// Wait for the offer to be pasted
	sd := mustReadStdin()

	// Set the remote SessionDescription
	answer := webrtc.RTCSessionDescription{
		Type: webrtc.RTCSdpTypeAnswer,
		Sdp:  sd,
	}

	// Apply the answer as the remote description
	err = peerConnection.SetRemoteDescription(answer)
	check(err)

	// Send messages every 5 seconds
	fmt.Println("Random messages will now be sent to any connected DataChannels every 5 seconds")
	for {
		time.Sleep(5 * time.Second)
		message := randSeq(15)
		fmt.Printf("Sending %s \n", message)

		err := dataChannel.Send(datachannel.PayloadString{Data: []byte(message)})
		check(err)
	}
}

// randSeq is used to generate a random message
func randSeq(n int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[r.Intn(len(letters))]
	}
	return string(b)
}

// mustReadStdin blocks untill input is received from stdin
func mustReadStdin() string {
	reader := bufio.NewReader(os.Stdin)
	rawSd, err := reader.ReadString('\n')
	check(err)

	fmt.Println("")
	sd, err := base64.StdEncoding.DecodeString(rawSd)

	check(err)
	return string(sd)
}

// check is used to panic in an error occurs.
func check(err error) {
	if err != nil {
		panic(err)
	}
}
