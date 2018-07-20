package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/pions/webrtc"
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

	/* Everything below is the pion-WebRTC API, thanks for using it! */

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

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	}

	datachannels := make([]*webrtc.RTCDataChannel, 0)
	var dataChannelsLock sync.RWMutex

	peerConnection.Ondatachannel = func(d *webrtc.RTCDataChannel) {
		dataChannelsLock.Lock()
		datachannels = append(datachannels, d)
		dataChannelsLock.Unlock()

		fmt.Printf("New DataChannel %s %d\n", d.Label, d.ID)
		d.Onmessage = func(message []byte) {
			fmt.Printf("Message from DataChannel %s '%s'\n", d.Label, string(message))
		}
	}

	// Set the remote SessionDescription
	offer := webrtc.RTCSessionDescription{
		Type: webrtc.RTCSdpTypeOffer,
		Sdp:  string(sd),
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
	fmt.Println("Random messages will now be sent to any connected DataChannels every 5 seconds")
	for {
		time.Sleep(5 * time.Second)
		message := randSeq(15)
		fmt.Printf("Sending %s \n", message)

		dataChannelsLock.RLock()
		for _, d := range datachannels {
			d.Send([]byte(message))
		}
		dataChannelsLock.RUnlock()
	}
}
