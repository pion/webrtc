// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build js && wasm
// +build js,wasm

package main

import (
	"fmt"
	"io"
	"syscall/js"
	"time"

	"github.com/pion/webrtc/v3"

	"github.com/pion/webrtc/v3/examples/internal/signal"
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

	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

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
		handleError(err)
	}

	// Create a datachannel with label 'data'
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		handleError(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log(fmt.Sprintf("ICE Connection State has changed: %s\n", connectionState.String()))
	})

	// Register channel opening handling
	dataChannel.OnOpen(func() {
		log(fmt.Sprintf("Data channel '%s'-'%d' open.\n", dataChannel.Label(), dataChannel.ID()))

		// Detach the data channel
		raw, dErr := dataChannel.Detach()
		if dErr != nil {
			handleError(dErr)
		}

		// Handle reading from the data channel
		go ReadLoop(raw)

		// Handle writing to the data channel
		go WriteLoop(raw)
	})

	// Create an offer to send to the browser
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		handleError(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		handleError(err)
	}

	// Add handlers for setting up the connection.
	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log(fmt.Sprint(state))
	})
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			encodedDescr := signal.Encode(peerConnection.LocalDescription())
			el := getElementByID("localSessionDescription")
			el.Set("value", encodedDescr)
		}
	})

	// Set up global callbacks which will be triggered on button clicks.
	/*js.Global().Set("sendMessage", js.FuncOf(func(_ js.Value, _ []js.Value) interface{} {
		go func() {
			el := getElementByID("message")
			message := el.Get("value").String()
			if message == "" {
				js.Global().Call("alert", "Message must not be empty")
				return
			}
			if err := sendChannel.SendText(message); err != nil {
				handleError(err)
			}
		}()
		return js.Undefined()
	}))*/
	js.Global().Set("startSession", js.FuncOf(func(_ js.Value, _ []js.Value) interface{} {
		go func() {
			el := getElementByID("remoteSessionDescription")
			sd := el.Get("value").String()
			if sd == "" {
				js.Global().Call("alert", "Session Description must not be empty")
				return
			}

			descr := webrtc.SessionDescription{}
			signal.Decode(sd, &descr)
			if err := peerConnection.SetRemoteDescription(descr); err != nil {
				handleError(err)
			}
		}()
		return js.Undefined()
	}))

	// Block forever
	select {}
}

// ReadLoop shows how to read from the datachannel directly
func ReadLoop(d io.Reader) {
	for {
		buffer := make([]byte, messageSize)
		n, err := d.Read(buffer)
		if err != nil {
			log(fmt.Sprintf("Datachannel closed; Exit the readloop: %v", err))
			return
		}

		log(fmt.Sprintf("Message from DataChannel: %s\n", string(buffer[:n])))
	}
}

// WriteLoop shows how to write to the datachannel directly
func WriteLoop(d io.Writer) {
	for range time.NewTicker(5 * time.Second).C {
		message := signal.RandSeq(messageSize)
		log(fmt.Sprintf("Sending %s \n", message))

		_, err := d.Write([]byte(message))
		if err != nil {
			handleError(err)
		}
	}
}

func log(msg string) {
	el := getElementByID("logs")
	el.Set("innerHTML", el.Get("innerHTML").String()+msg+"<br>")
}

func handleError(err error) {
	log("Unexpected error. Check console.")
	panic(err)
}

func getElementByID(id string) js.Value {
	return js.Global().Get("document").Call("getElementById", id)
}
