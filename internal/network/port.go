package network

import (
	"net"
	"sync"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/pkg/ice"
	"golang.org/x/net/ipv4"
)

type port struct {
	iceState ice.ConnectionState

	conn          *ipv4.PacketConn
	listeningAddr *stun.TransportAddr

	seenPeers     map[string]*net.UDPAddr
	seenPeersLock sync.RWMutex

	m *Manager
}

func newPort(address string, m *Manager) (*port, error) {
	listener, err := net.ListenPacket("udp4", address)
	if err != nil {
		return nil, err
	}

	addr, err := stun.NewTransportAddr(listener.LocalAddr())
	if err != nil {
		return nil, err
	}

	conn := ipv4.NewPacketConn(listener)
	dtls.AddListener(addr.String(), conn)

	p := &port{
		listeningAddr: addr,
		conn:          conn,
		m:             m,
		seenPeers:     make(map[string]*net.UDPAddr),
	}

	go p.networkLoop()
	return p, nil
}

func (p *port) close() error {
	return p.conn.Close()
}
