// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/pion/webrtc/v4"
)

// nolint:cyclop
func setupAnsweringAgent() {
	// Create and start a simple HTTP proxy server.
	proxyURL := newHTTPProxy()
	// Create a proxy dialer that will use the created HTTP proxy.
	proxyDialer := newProxyDialer(proxyURL)

	var settingEngine webrtc.SettingEngine
	// Set the ICEProxyDialer to use the proxy for TURN+TCP connections.
	settingEngine.SetICEProxyDialer(proxyDialer)
	api := webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine))

	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs:       []string{turnServerURL},
				Username:   turnUsername,
				Credential: turnPassword,
			},
		},
		// ICETransportPolicyRelay forces the connection to go through a TURN server.
		// This is required for the proxy to be used.
		ICETransportPolicy: webrtc.ICETransportPolicyRelay,
	})
	if err != nil {
		panic(err)
	}

	// Log peer connection and ICE connection state changes.
	peerConnection.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		log.Printf("[Answerer] Peer Connection State has changed: %s", pcs.String())
	})
	peerConnection.OnICEConnectionStateChange(func(ics webrtc.ICEConnectionState) {
		log.Printf("[Answerer] ICE Connection State has changed: %s", ics.String())
	})

	// Register a handler for when a data channel is created by the remote peer.
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		icePair, err := d.Transport().Transport().ICETransport().GetSelectedCandidatePair()
		if err != nil {
			panic(err)
		}
		// Log the chosen ICE candidate pair.
		log.Printf("[Answerer] New DataChannel %s, ICE pair: (%s)<->(%s)",
			d.Label(), icePair.Local.String(), icePair.Remote.String())
		// Register a handler to echo messages back to the sender.
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			if err := d.SendText(string(msg.Data)); err != nil {
				panic(err)
			}
		})
	})

	// HTTP handler that accepts an offer, creates an answer,
	// and sends it back to the offering agent.
	http.HandleFunc("/sdp", func(rw http.ResponseWriter, r *http.Request) {
		var sdp webrtc.SessionDescription
		if err := json.NewDecoder(r.Body).Decode(&sdp); err != nil {
			panic(err)
		}

		if err := peerConnection.SetRemoteDescription(sdp); err != nil {
			panic(err)
		}

		gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			panic(err)
		}

		if err = peerConnection.SetLocalDescription(answer); err != nil {
			panic(err)
		}

		// Block until ICE Gathering is complete, disabling trickle ICE
		// we do this because we only can exchange one signaling message
		// in a production application you should exchange ICE Candidates via OnICECandidate
		<-gatherComplete

		resp, err := json.Marshal(*peerConnection.LocalDescription())
		if err != nil {
			panic(err)
		}

		if _, err := rw.Write(resp); err != nil {
			panic(err)
		}
	})

	// Start an HTTP server to handle the SDP exchange from the offering agent.
	go func() {
		// The HTTP server is not gracefully shutdown in this example.
		// nolint:gosec
		err := http.ListenAndServe("localhost:8080", nil)
		if err != nil {
			panic(err)
		}
	}()
}
