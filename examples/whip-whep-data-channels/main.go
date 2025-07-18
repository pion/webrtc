// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// whip-whep demonstrates how to use the WHIP/WHEP specifications to exchange SPD descriptions
// and stream media to a WebRTC client in the browser or OBS.
package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/pion/webrtc/v4"
)

// nolint: gochecknoglobals
var (
	peerConnectionConfiguration = webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
)

// nolint:gocognit
func main() {
	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.
	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/whep", whepHandler)
	http.HandleFunc("/whip", whipHandler)

	fmt.Println("Open http://localhost:8080 to access this demo")
	panic(http.ListenAndServe(":8080", nil)) // nolint: gosec
}

func whipHandler(res http.ResponseWriter, req *http.Request) {
	// Read the offer from HTTP Request
	offer, err := io.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfiguration)
	if err != nil {
		panic(err)
	}

	// Send answer via HTTP Response
	writeAnswer(res, peerConnection, offer, "/whip")
}

func whepHandler(res http.ResponseWriter, req *http.Request) {
	// Read the offer from HTTP Request
	offer, err := io.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfiguration)
	if err != nil {
		panic(err)
	}

	// Send answer via HTTP Response
	writeAnswer(res, peerConnection, offer, "/whep")
}

func writeAnswer(res http.ResponseWriter, peerConnection *webrtc.PeerConnection, offer []byte, path string) {
	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())

		if connectionState == webrtc.ICEConnectionStateFailed {
			_ = peerConnection.Close()
		}
	})

	if err := peerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer, SDP: string(offer),
	}); err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// WHIP+WHEP expects a Location header and a HTTP Status Code of 201
	res.Header().Add("Location", path)
	res.WriteHeader(http.StatusCreated)

	// Write Answer with Candidates as HTTP Response
	fmt.Fprint(res, peerConnection.LocalDescription().SDP) //nolint: errcheck
}
