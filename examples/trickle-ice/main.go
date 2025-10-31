// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// trickle-ice demonstrates Pion WebRTC's Trickle ICE APIs.  ICE is the subsystem WebRTC uses to establish connectivity.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

// websocketServer is called for every new inbound WebSocket
// nolint: gocognit, cyclop
func websocketServer(wsConn *websocket.Conn) {
	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	// When Pion gathers a new ICE Candidate send it to the client. This is how
	// ice trickle is implemented. Everytime we have a new candidate available we send
	// it as soon as it is ready. We don't wait to emit a Offer/Answer until they are
	// all available
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		outbound, marshalErr := json.Marshal(candidate.ToJSON())
		if marshalErr != nil {
			fmt.Println("Marshal ICECandidate error:", marshalErr)
			return
		}

		if _, err = wsConn.Write(outbound); err != nil {
			fmt.Println("WebSocket write error:", err)
			return
		}
	})

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Send the current time via a DataChannel to the remote peer every 3 seconds
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		d.OnOpen(func() {
			fmt.Println(time.Now().Format("15:04:05"), "- DataChannel open")
			// Periodically send timestamped messages
			go func() {
				ticker := time.NewTicker(time.Second * 3)
				defer ticker.Stop()
				for range ticker.C {
					if err := d.SendText(time.Now().String()); err != nil {
						fmt.Println(time.Now().Format("15:04:05"), "- DataChannel closed, stopping send loop")
						return
					}
				}
			}()
		})
	})

	buf := make([]byte, 1500)
	for {
		// Read each inbound WebSocket Message
		n, err := wsConn.Read(buf)
		if err != nil {
			fmt.Println(time.Now().Format("15:04:05"), "- WebSocket read error:", err)
			return

		}

		// Unmarshal each inbound WebSocket message
		var (
			candidate webrtc.ICECandidateInit
			offer     webrtc.SessionDescription
		)

		switch {
		// Attempt to unmarshal as a SessionDescription. If the SDP field is empty
		// assume it is not one.
		case json.Unmarshal(buf[:n], &offer) == nil && offer.SDP != "":
			if err = peerConnection.SetRemoteDescription(offer); err != nil {
				fmt.Println("SetRemoteDescription error:", err)
				return
			}

			answer, answerErr := peerConnection.CreateAnswer(nil)
			if answerErr != nil {
				fmt.Println("CreateAnswer error:", err)
				return
			}

			if err = peerConnection.SetLocalDescription(answer); err != nil {
				fmt.Println("SetLocalDescription error:", err)
				return
			}

			outbound, marshalErr := json.Marshal(answer)
			if marshalErr != nil {
				fmt.Println("Marshal answer error:", err)
				return
			}

			if _, err = wsConn.Write(outbound); err != nil {
				fmt.Println("WebSocket write error:", err)
				return

			}
		// Attempt to unmarshal as a ICECandidateInit. If the candidate field is empty
		// assume it is not one.
		case json.Unmarshal(buf[:n], &candidate) == nil && candidate.Candidate != "":
			if err = peerConnection.AddICECandidate(candidate); err != nil {
				fmt.Println("AddICECandidate error:", err)
				return
			}
		default:
			fmt.Println("Unknown WebSocket message")
		}
	}
}

func main() {
	http.Handle("/", http.FileServer(http.Dir(".")))
	http.Handle("/websocket", websocket.Handler(websocketServer))

	fmt.Println("Open http://localhost:8080 to access this demo")
	// nolint: gosec
	panic(http.ListenAndServe(":8080", nil))
}
