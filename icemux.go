// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"net"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/logging"
)

// NewICETCPMux creates a new instance of ice.TCPMuxDefault. It enables use of
// passive ICE TCP candidates.
func NewICETCPMux(logger logging.LeveledLogger, listener net.Listener, readBufferSize int) ice.TCPMux {
	return ice.NewTCPMuxDefault(ice.TCPMuxParams{
		Listener:       listener,
		Logger:         logger,
		ReadBufferSize: readBufferSize,
	})
}

// NewICEUDPMux creates a new instance of ice.UDPMuxDefault. It allows many PeerConnections to be served
// by a single UDP Port.
func NewICEUDPMux(logger logging.LeveledLogger, udpConn net.PacketConn) ice.UDPMux {
	return ice.NewUDPMuxDefault(ice.UDPMuxParams{
		UDPConn: udpConn,
		Logger:  logger,
	})
}

// NewICEUniversalUDPMux creates a new instance of ice.UniversalUDPMux. It allows many PeerConnections
// with host, server reflexive and relayed candidates to be served by a single UDP port. Pass the
// returned mux to both SettingEngine.SetICEUDPMux and SettingEngine.SetICEUDPMuxSrflx to also
// multiplex the STUN traffic used for server reflexive candidate gathering.
//
// xorMappedAddrCacheTTL controls how long a discovered server reflexive (XOR-mapped) address is
// cached before another STUN binding request is issued.
func NewICEUniversalUDPMux(
	logger logging.LeveledLogger,
	udpConn net.PacketConn,
	xorMappedAddrCacheTTL time.Duration,
) ice.UniversalUDPMux {
	return ice.NewUniversalUDPMuxDefault(ice.UniversalUDPMuxParams{
		Logger:                logger,
		UDPConn:               udpConn,
		XORMappedAddrCacheTTL: xorMappedAddrCacheTTL,
	})
}
