package network

import (
	"net"
	"strconv"
	"sync"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtp"
	"golang.org/x/net/ipv4"
)

type authedConnection struct {
	pair *dtls.CertPair
	peer net.Addr
}

// Port represents a UDP listener that handles incoming/outgoing traffic
type Port struct {
	ListeningAddr *stun.TransportAddr
	ICEState      ice.ConnectionState

	dtlsStates map[string]*dtls.State

	authedConnectionsLock *sync.Mutex
	authedConnections     []*authedConnection

	bufferTransports map[uint32]chan<- *rtp.Packet

	// https://tools.ietf.org/html/rfc3711#section-3.2.3
	// A cryptographic context SHALL be uniquely identified by the triplet
	//  <SSRC, destination network address, destination transport port number>
	// contexts are keyed by IP:PORT:SSRC
	srtpContextsLock *sync.Mutex
	srtpContexts     map[string]*srtp.Context

	conn *ipv4.PacketConn
}

// NewPort creates a new Port
func NewPort(address string, remoteKey []byte, tlscfg *dtls.TLSCfg, b BufferTransportGenerator, i ICENotifier) (*Port, error) {
	listener, err := net.ListenPacket("udp4", address)
	if err != nil {
		return nil, err
	}

	addr, err := stun.NewTransportAddr(listener.LocalAddr())
	if err != nil {
		return nil, err
	}

	srcString := addr.IP.String() + ":" + strconv.Itoa(addr.Port)
	conn := ipv4.NewPacketConn(listener)
	dtls.AddListener(srcString, conn)

	p := &Port{
		ListeningAddr:         addr,
		conn:                  conn,
		dtlsStates:            make(map[string]*dtls.State),
		bufferTransports:      make(map[uint32]chan<- *rtp.Packet),
		authedConnectionsLock: &sync.Mutex{},

		srtpContextsLock: &sync.Mutex{},
		srtpContexts:     make(map[string]*srtp.Context),
	}

	go p.networkLoop(remoteKey, tlscfg, b, i)
	return p, nil
}

// Close closes the listening port and cleans up any state
func (p *Port) Close() error {
	return p.conn.Close()
}
