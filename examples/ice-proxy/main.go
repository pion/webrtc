// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// ice-proxy demonstrates Pion WebRTC's proxy abilities.
package main

const (
	turnServerAddr = "localhost:17342"
	turnServerURL  = "turn:" + turnServerAddr + "?transport=tcp"
	turnUsername   = "turn_username"
	turnPassword   = "turn_password"
)

func main() {
	// Setup TURN server.
	turnServer := newTURNServer()
	defer turnServer.Close() // nolint:errcheck

	// Setup answering agent with proxy and TURN.
	setupAnsweringAgent()
	// Setup offering agent with only direct communication.
	setupOfferingAgent()

	// Block forever
	select {}
}
