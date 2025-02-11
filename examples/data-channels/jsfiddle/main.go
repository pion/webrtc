// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build js && wasm
// +build js,wasm

package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall/js"

	"github.com/pion/webrtc/v4"
)

func main() {
	// Configure and create a new PeerConnection.
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		handleError(err)
	}

	// Create DataChannel.
	sendChannel, err := pc.CreateDataChannel("foo", nil)
	if err != nil {
		handleError(err)
	}
	sendChannel.OnClose(func() {
		fmt.Println("sendChannel has closed")
	})
	sendChannel.OnClosing(func() {
		fmt.Println("sendChannel is closing")
	})
	sendChannel.OnError(func(err error) {
		fmt.Println("sendChannel error", err)
	})
	sendChannel.OnOpen(func() {
		fmt.Println("sendChannel has opened")

		candidatePair, err := pc.SCTP().Transport().ICETransport().GetSelectedCandidatePair()

		fmt.Println(candidatePair)
		fmt.Println(err)
	})
	sendChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		log(fmt.Sprintf("Message from DataChannel %s payload %s", sendChannel.Label(), string(msg.Data)))
	})

	// Create offer
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		handleError(err)
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		handleError(err)
	}

	// Add handlers for setting up the connection.
	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log(fmt.Sprint(state))
	})
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			encodedDescr := encode(pc.LocalDescription())
			el := getElementByID("localSessionDescription")
			el.Set("value", encodedDescr)
		}
	})

	// Set up global callbacks which will be triggered on button clicks.
	js.Global().Set("sendMessage", js.FuncOf(func(_ js.Value, _ []js.Value) interface{} {
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
	}))
	js.Global().Set("startSession", js.FuncOf(func(_ js.Value, _ []js.Value) interface{} {
		go func() {
			el := getElementByID("remoteSessionDescription")
			sd := el.Get("value").String()
			if sd == "" {
				js.Global().Call("alert", "Session Description must not be empty")
				return
			}

			descr := webrtc.SessionDescription{}
			decode(sd, &descr)
			if err := pc.SetRemoteDescription(descr); err != nil {
				handleError(err)
			}
		}()
		return js.Undefined()
	}))
	js.Global().Set("copySDP", js.FuncOf(func(_ js.Value, _ []js.Value) interface{} {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					switch e := e.(type) {
					case error:
						handleError(e)
					default:
						handleError(fmt.Errorf("recovered with non-error value: (%T) %s", e, e))
					}
				}
			}()

			browserSDP := getElementByID("localSessionDescription")

			browserSDP.Call("focus")
			browserSDP.Call("select")

			copyStatus := js.Global().Get("document").Call("execCommand", "copy")
			if copyStatus.Bool() {
				log("Copying SDP was successful")
			} else {
				log("Copying SDP was unsuccessful")
			}
		}()
		return js.Undefined()
	}))

	// Stay alive
	select {}
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

// Read from stdin until we get a newline
func readUntilNewline() (in string) {
	var err error

	r := bufio.NewReader(os.Stdin)
	for {
		in, err = r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			panic(err)
		}

		if in = strings.TrimSpace(in); len(in) > 0 {
			break
		}
	}

	fmt.Println("")
	return
}

// JSON encode + base64 a SessionDescription
func encode(obj *webrtc.SessionDescription) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode a base64 and unmarshal JSON into a SessionDescription
func decode(in string, obj *webrtc.SessionDescription) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}

	if err = json.Unmarshal(b, obj); err != nil {
		panic(err)
	}
}
