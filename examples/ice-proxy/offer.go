// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/pion/webrtc/v4"
)

// nolint:cyclop
func setupOfferingAgent() {
	var settingEngine webrtc.SettingEngine
	// Allow loopback candidates.
	settingEngine.SetIncludeLoopbackCandidate(true)
	api := webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine))

	// Create a new RTCPeerConnection.
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	// Log peer connection and ICE connection state changes.
	peerConnection.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		log.Printf("[Offerer] Peer Connection State has changed: %s", pcs.String())
	})
	peerConnection.OnICEConnectionStateChange(func(ics webrtc.ICEConnectionState) {
		log.Printf("[Offerer] ICE Connection State has changed: %s", ics.String())
	})

	// Create a data channel for measuring round-trip time.
	dc, err := peerConnection.CreateDataChannel("data-channel", nil)
	if err != nil {
		panic(err)
	}
	dc.OnOpen(func() {
		// Send the current time every 3 seconds.
		for range time.Tick(3 * time.Second) {
			if sendErr := dc.SendText(time.Now().Format(time.RFC3339Nano)); sendErr != nil {
				panic(sendErr)
			}
		}
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		// Receive the echoed time from the remote agent and calculate the round-trip time.
		sendTime, parseErr := time.Parse(time.RFC3339Nano, string(msg.Data))
		if parseErr != nil {
			panic(parseErr)
		}
		log.Printf("[Offerer] Data channel round-trip time: %s", time.Since(sendTime))
	})

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Create an offer to send to the answering agent.
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	if err = peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE.
	// We do this because we only can exchange one signaling message.
	// In a production application you should exchange ICE Candidates via OnICECandidate.
	<-gatherComplete

	offerJSON, err := json.Marshal(*peerConnection.LocalDescription())
	if err != nil {
		panic(err)
	}

	// Send offer to the answering agent.
	// nolint:noctx
	resp, err := http.Post("http://localhost:8080/sdp", "application/json", bytes.NewBuffer(offerJSON))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close() // nolint:errcheck

	// Receive answer and set remote description.
	var answer webrtc.SessionDescription
	if err = json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		panic(err)
	}
	if err = peerConnection.SetRemoteDescription(answer); err != nil {
		panic(err)
	}
}
