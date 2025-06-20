// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// ice-proxy demonstrates Pion WebRTC's proxy abilities.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/pion/webrtc/v4"
)

var api *webrtc.API //nolint

//nolint:cyclop

func doSignaling(res http.ResponseWriter, req *http.Request) { //nolint:cyclop
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs:       []string{"turn:127.0.0.1:17342?transport=tcp"},
				Username:   "turn_username",
				Credential: "turn_password",
			},
		},
	})
	if err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Send the current time via a DataChannel to the remote peer every 3 seconds
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		d.OnOpen(func() {
			for range time.Tick(time.Second * 3) {
				if err = d.SendText(time.Now().String()); err != nil {
					if errors.Is(err, io.ErrClosedPipe) {
						return
					}
					panic(err)
				}
			}
		})
	})

	var offer webrtc.SessionDescription
	if err = json.NewDecoder(req.Body).Decode(&offer); err != nil {
		panic(err)
	}

	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

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

	response, err := json.Marshal(*peerConnection.LocalDescription())
	if err != nil {
		panic(err)
	}

	res.Header().Set("Content-Type", "application/json")
	if _, err := res.Write(response); err != nil {
		panic(err)
	}
}

//nolint:cyclop
func main() {
	// Setup TURN
	turnServer, err := newTURNServer()
	if err != nil {
		panic(err)
	}
	defer turnServer.Close()

	// Setup proxy
	proxyURL, proxyListener, err := newHTTPProxy()
	if err != nil {
		panic(err)
	}
	defer proxyListener.Close()

	// Setup proxy dialer
	proxyDialer, err := newProxyDialer(proxyURL)
	if err != nil {
		panic(err)
	}

	// Set proxy dialer, works only for TURN + TCP
	var settingEngine webrtc.SettingEngine
	settingEngine.SetICEProxyDialer(proxyDialer)

	api = webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine))

	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/doSignaling", doSignaling)

	fmt.Println("Open http://localhost:8080 to access this demo")
	// nolint: gosec
	panic(http.ListenAndServe(":8080", nil))
}
