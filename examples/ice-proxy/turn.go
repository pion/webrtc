// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package main

import (
	"net"

	"github.com/pion/turn/v4"
)

func newTURNServer() *turn.Server {
	tcpListener, err := net.Listen("tcp4", turnServerAddr) // nolint: noctx
	if err != nil {
		panic(err)
	}

	server, err := turn.NewServer(turn.ServerConfig{
		AuthHandler: func(_, realm string, _ net.Addr) ([]byte, bool) {
			// Accept any request with provided username and password.
			return turn.GenerateAuthKey(turnUsername, realm, turnPassword), true
		},
		ListenerConfigs: []turn.ListenerConfig{
			{
				Listener: tcpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorNone{
					Address: "localhost",
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	return server
}
