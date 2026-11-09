// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"io"
	"net"
	"time"

	"github.com/pion/ice/v5"
	"github.com/pion/logging"
)

// ICETCPMux multiplexes ICE connections over a shared TCP listener.
type ICETCPMux interface {
	io.Closer
	GetConnByUfrag(ufrag string, isIPv6 bool, local net.IP) (net.PacketConn, error)
	RemoveConnByUfrag(ufrag string)
}

// ICEUDPMux multiplexes ICE connections over shared UDP sockets.
type ICEUDPMux interface {
	io.Closer
	GetConn(ufrag string, addr net.Addr) (net.PacketConn, error)
	RemoveConnByUfrag(ufrag string)
	GetListenAddresses() []net.Addr
}

// NewICETCPMux creates a new instance of ice.TCPMuxDefault. It enables use of
// passive ICE TCP candidates.
func NewICETCPMux(logger logging.LeveledLogger, listener net.Listener, readBufferSize int) ICETCPMux {
	return ice.NewTCPMuxDefault(ice.TCPMuxParams{
		Listener:        listener,
		Logger:          logger,
		ReadBufferSize:  readBufferSize,
		WriteBufferSize: 4 * 1024 * 1024, // 4 MB recommended by pion/ice.
	})
}

// NewICEUDPMux creates a new instance of ice.UDPMuxDefault. It allows many PeerConnections to be served
// by a single UDP Port.
func NewICEUDPMux(logger logging.LeveledLogger, udpConn net.PacketConn) ICEUDPMux {
	return ice.NewUDPMuxDefault(ice.UDPMuxParams{
		UDPConn: udpConn,
		Logger:  logger,
	})
}

// NewICEUniversalUDPMux creates a new instance of ice.UniversalUDPMux. It allows many PeerConnections
// with host and server reflexive candidates to be served by a single UDP port. Pass the
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
