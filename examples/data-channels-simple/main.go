// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// simple-datachannel is a simple datachannel demo that auto connects.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pion/webrtc/v4"
)

func main() {
	var pc *webrtc.PeerConnection

	setupOfferHandler(&pc)
	setupCandidateHandler(&pc)
	setupStaticHandler()

	fmt.Println("🚀 Signaling server started on http://localhost:8080")
	//nolint:gosec
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
	}
}

func setupOfferHandler(pc **webrtc.PeerConnection) {
	http.HandleFunc("/offer", func(responseWriter http.ResponseWriter, r *http.Request) {
		var offer webrtc.SessionDescription
		if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
			http.Error(responseWriter, err.Error(), http.StatusBadRequest)

			return
		}

		// PeerConnection with enhanced configuration for better browser compatibility
		var err error
		*pc, err = webrtc.NewPeerConnection(webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{URLs: []string{"stun:stun.l.google.com:19302"}},
			},
			BundlePolicy:  webrtc.BundlePolicyBalanced,
			RTCPMuxPolicy: webrtc.RTCPMuxPolicyRequire,
		})
		if err != nil {
			http.Error(responseWriter, err.Error(), http.StatusInternalServerError)

			return
		}

		setupICECandidateHandler(*pc)
		setupDataChannelHandler(*pc)

		if err := processOffer(*pc, offer, responseWriter); err != nil {
			http.Error(responseWriter, err.Error(), http.StatusInternalServerError)

			return
		}
	})
}

func setupICECandidateHandler(pc *webrtc.PeerConnection) {
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			fmt.Printf("🌐 New ICE candidate: %s\n", c.Address)
		}
	})
}

func setupDataChannelHandler(pc *webrtc.PeerConnection) {
	pc.OnDataChannel(func(d *webrtc.DataChannel) {
		d.OnOpen(func() {
			fmt.Println("✅ DataChannel opened (Server)")
			if sendErr := d.SendText("Hello from Go server 👋"); sendErr != nil {
				fmt.Printf("Failed to send text: %v\n", sendErr)
			}
		})
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("📩 Received: %s\n", string(msg.Data))
		})
	})
}

func processOffer(
	pc *webrtc.PeerConnection,
	offer webrtc.SessionDescription,
	responseWriter http.ResponseWriter,
) error {
	// Set remote description
	if err := pc.SetRemoteDescription(offer); err != nil {
		return err
	}

	// Create answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return err
	}

	// Set local description
	if err := pc.SetLocalDescription(answer); err != nil {
		return err
	}

	// Wait for ICE gathering to complete before sending answer
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	<-gatherComplete

	finalAnswer := pc.LocalDescription()
	if finalAnswer == nil {
		//nolint:err113
		return fmt.Errorf("local description is nil after ICE gathering")
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(responseWriter).Encode(*finalAnswer); err != nil {
		fmt.Printf("Failed to encode answer: %v\n", err)
	}

	return nil
}

func setupCandidateHandler(pc **webrtc.PeerConnection) {
	http.HandleFunc("/candidate", func(responseWriter http.ResponseWriter, r *http.Request) {
		var candidate webrtc.ICECandidateInit
		if err := json.NewDecoder(r.Body).Decode(&candidate); err != nil {
			http.Error(responseWriter, err.Error(), http.StatusBadRequest)

			return
		}
		if *pc != nil {
			if err := (*pc).AddICECandidate(candidate); err != nil {
				fmt.Println("Failed to add candidate", err)
			}
		}
	})
}

func setupStaticHandler() {
	// demo.html
	http.HandleFunc("/", func(responseWriter http.ResponseWriter, r *http.Request) {
		http.ServeFile(responseWriter, r, "./demo.html")
	})
}
