// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// WARP (SNAP+SPED) testbed.
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

	fmt.Println("google-chrome-unstable --force-fieldtrials=" +
		"WebRTC-Sctp-Snap/Enabled/WebRTC-IceHandshakeDtls/Enabled/ " +
		"--disable-features=WebRtcPqcForDtls http://localhost:8080")
	fmt.Printf("Add `--enable-logging --v=1` and then " +
		"`grep SCTP_PACKET chrome_debug.log | " +
		"text2pcap -D -u 1001,2001 -t \"%%H:%%M:%%S.%%f\" - out.pcap` " +
		"for inspecting the raw packets.\n")
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

		var err error
		*pc, err = webrtc.NewPeerConnection(webrtc.Configuration{
			BundlePolicy: webrtc.BundlePolicyMaxBundle,
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
			if sendErr := d.SendText("ECHO " + string(msg.Data)); sendErr != nil {
				fmt.Printf("Failed to send text: %v\n", sendErr)
			}
		})
	})
	if serverDc, err := pc.CreateDataChannel("server-opened-channel", nil); err == nil {
		serverDc.OnOpen(func() {
			if sendErr := serverDc.SendText("Server opened channel ready"); sendErr != nil {
				fmt.Printf("Failed to send on server-opened channel: %v\n", sendErr)
			}
		})
	}
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
	http.HandleFunc("/", func(responseWriter http.ResponseWriter, r *http.Request) {
		http.ServeFile(responseWriter, r, "./index.html")
	})
}
