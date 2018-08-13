package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
)

func randSeq(n int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[r.Intn(len(letters))]
	}
	return string(b)
}

func main() {
	// Create a new RTCPeerConnection
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

	d, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		// TODO: find the correct place of this
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == ice.ConnectionStateConnected {
			fmt.Println("sending openchannel")
			err := d.SendOpenChannelMessage()
			if err != nil {
				fmt.Println("faild to send openchannel", err)
			}
		}
	}

	fmt.Printf("New DataChannel %s %d\n", d.Label, d.ID)

	d.Lock()
	d.Onmessage = func(payload datachannel.Payload) {
		switch p := payload.(type) {
		case *datachannel.PayloadString:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '%s'\n", p.PayloadType().String(), d.Label, string(p.Data))
		case *datachannel.PayloadBinary:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '% 02x'\n", p.PayloadType().String(), d.Label, p.Data)
		default:
			fmt.Printf("Message '%s' from DataChannel '%s' no payload \n", p.PayloadType().String(), d.Label)
		}
	}
	d.Unlock()

	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	/* Signaling via STDIN */

	// Get the LocalDescription and take it to base64 so we can paste in browser
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(offer.Sdp)))
	sd := mustReadStdin()

	/* --- */

	// Set the remote SessionDescription
	answer := webrtc.RTCSessionDescription{
		Type: webrtc.RTCSdpTypeAnswer,
		Sdp:  sd,
	}
	if err := peerConnection.SetRemoteDescription(answer); err != nil {
		panic(err)
	}

	fmt.Println("Random messages will now be sent to any connected DataChannels every 5 seconds")
	for {
		time.Sleep(5 * time.Second)
		message := randSeq(15)
		fmt.Printf("Sending %s \n", message)

		err := d.Send(datachannel.PayloadString{Data: []byte(message)})
		if err != nil {
			panic(err)
		}
	}
}

func mustReadStdin() string {
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
	return string(sd)
}
