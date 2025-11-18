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
	"math/rand"
	"net/http"
	"sync"

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

	// Broadcast hub to forward messages between all connected clients.
	broadcastHub = &Hub{
		connections: make(map[*webrtc.DataChannel]bool),
		usernames:   make(map[*webrtc.DataChannel]string),
		mu:          sync.RWMutex{},
	}
)

// Hub manages all connected DataChannels for broadcasting.
type Hub struct {
	connections map[*webrtc.DataChannel]bool
	usernames   map[*webrtc.DataChannel]string
	mu          sync.RWMutex
}

// nolint: gochecknoglobals
var (
	adjectives = []string{
		"Quick", "Swift", "Bright", "Bold", "Calm", "Cool", "Fast", "Happy",
		"Lucky", "Shy", "Sneaky", "Wise", "Brave", "Clever", "Kind", "Proud",
	}
	nouns = []string{
		"Fox", "Eagle", "Lion", "Tiger", "Wolf", "Dragon", "Hawk", "Bear",
		"Shark", "Falcon", "Leopard", "Panther", "Phoenix", "Raven", "Crow", "Owl",
	}
)

// Register adds a DataChannel to the broadcast hub and assigns a random username.
func (h *Hub) Register(channel *webrtc.DataChannel) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[channel] = true

	username := h.generateUniqueUsername()
	h.usernames[channel] = username

	return username
}

// Unregister removes a DataChannel from the broadcast hub.
func (h *Hub) Unregister(channel *webrtc.DataChannel) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.connections, channel)
	delete(h.usernames, channel)
}

// generateUniqueUsername generates a unique username by combining an adjective and a noun.
// It checks existing usernames and regenerates until it finds a unique one.
// Must be called while holding h.mu.Lock().
// nolint: gosec
func (h *Hub) generateUniqueUsername() string {
	var username string
	for {
		adjective := adjectives[rand.Intn(len(adjectives))]
		noun := nouns[rand.Intn(len(nouns))]
		number := rand.Intn(1000)
		username = fmt.Sprintf("%s%s%d", adjective, noun, number)

		// Check if this username already exists by iterating over map values directly
		exists := false
		for _, existingUsername := range h.usernames {
			if existingUsername == username {
				exists = true

				break
			}
		}

		if !exists {
			break
		}
	}

	return username
}

// GetUsername returns the username for a DataChannel.
func (h *Hub) GetUsername(channel *webrtc.DataChannel) string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.usernames[channel]
}

// Broadcast sends a message to all registered DataChannels including the sender.
func (h *Hub) Broadcast(message string, sender *webrtc.DataChannel) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Get the sender's username
	senderUsername := h.usernames[sender]
	formattedMessage := fmt.Sprintf("%s: %s", senderUsername, message)

	for channel := range h.connections {
		// Check if channel is still open
		if channel.ReadyState() != webrtc.DataChannelStateOpen {
			continue
		}

		// Send message in goroutine to avoid blocking
		go func(ch *webrtc.DataChannel, msg string) {
			if err := ch.SendText(msg); err != nil {
				fmt.Printf("Failed to send broadcast message: %v\n", err)
			}
		}(channel, formattedMessage)
	}
}

// Count returns the number of connected clients.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.connections)
}

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

		if connectionState == webrtc.ICEConnectionStateFailed ||
			connectionState == webrtc.ICEConnectionStateClosed {
			_ = peerConnection.Close()
		}
	})

	peerConnection.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", dataChannel.Label(), dataChannel.ID())

		dataChannel.OnOpen(func() {
			// register this channel in the broadcast hub and get assigned username
			username := broadcastHub.Register(dataChannel)
			fmt.Printf("Data channel '%s'-'%d' opened. Username: %s, Total clients: %d\n",
				dataChannel.Label(), dataChannel.ID(), username, broadcastHub.Count())
		})

		dataChannel.OnClose(func() {
			fmt.Printf("Data channel '%s'-'%d' closed\n", dataChannel.Label(), dataChannel.ID())
			// unregister this channel from the broadcast hub
			broadcastHub.Unregister(dataChannel)
		})

		dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			message := string(msg.Data)
			fmt.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label(), message)

			// broadcast the message to all other connected clients
			broadcastHub.Broadcast(message, dataChannel)
		})
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
