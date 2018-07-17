package network

import (
	"net"
	"strconv"
	"sync"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/sctp"
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

	dtlsStates      map[string]*dtls.State
	sctpAssocations map[string]*sctp.Association

	authedConnectionsLock *sync.Mutex
	authedConnections     []*authedConnection

	bufferTransports map[uint32]chan<- *rtp.Packet

	// https://tools.ietf.org/html/rfc3711#section-3.2.3
	// A cryptographic context SHALL be uniquely identified by the triplet
	//  <SSRC, destination network address, destination transport port number>
	// contexts are keyed by IP:PORT:SSRC
	srtpContextsLock *sync.Mutex
	srtpContexts     map[string]*srtp.Context

	association *sctp.Association

	conn     *ipv4.PacketConn
	certPair *dtls.CertPair
}

// PortArguments are all the mandatory arguments when creating a new port for send/recv of traffic
type PortArguments struct {
	Address   string
	RemoteKey []byte
	TLSCfg    *dtls.TLSCfg

	BufferTransportGenerator BufferTransportGenerator

	ICENotifier ICENotifier

	DataChannelEventHandler DataChannelEventHandler
}

// NewPort creates a new Port
func NewPort(args *PortArguments) (*Port, error) {
	listener, err := net.ListenPacket("udp4", args.Address)
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
		ListeningAddr:   addr,
		conn:            conn,
		dtlsStates:      make(map[string]*dtls.State),
		sctpAssocations: make(map[string]*sctp.Association),

		bufferTransports:      make(map[uint32]chan<- *rtp.Packet),
		authedConnectionsLock: &sync.Mutex{},

		srtpContextsLock: &sync.Mutex{},
		srtpContexts:     make(map[string]*srtp.Context),
	}

	go p.networkLoop(args.RemoteKey, args.TLSCfg, args.BufferTransportGenerator, args.ICENotifier, args.DataChannelEventHandler)
	return p, nil
}

// Close closes the listening port and cleans up any state
func (p *Port) Close() error {
	return p.conn.Close()
}
