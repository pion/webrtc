package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/srtp"
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
func NewPort(address string, remoteKey []byte, tlscfg *dtls.TLSCfg, b BufferTransportGenerator) (*Port, error) {
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

	go p.networkLoop(srcString, remoteKey, tlscfg, b)
	return p, nil
}

// Stop closes the listening port and cleans up any state
func (p *Port) Stop() {
}

// Send sends a *rtp.Packet if we have a connected peer
func (p *Port) Send(packet *rtp.Packet) {
	var err error

	for _, authed := range p.authedConnections {

		contextMapKey := authed.peer.String() + ":" + fmt.Sprint(packet.SSRC)
		p.srtpContextsLock.Lock()
		srtpContext, ok := p.srtpContexts[contextMapKey]
		if !ok {
			srtpContext, err = srtp.CreateContext([]byte(authed.pair.ClientWriteKey[0:16]), []byte(authed.pair.ClientWriteKey[16:]), authed.pair.Profile, 2581832418)
			if err != nil {
				fmt.Println("Failed to build SRTP context")
				continue
			}

			p.srtpContexts[contextMapKey] = srtpContext
		}
		p.srtpContextsLock.Unlock()

		if ok := srtpContext.EncryptPacket(packet); ok {
			raw, err := packet.Marshal()
			if err != nil {
				fmt.Printf("Failed to marshal packet: %s \n", err.Error())
			}
			if _, err := p.conn.WriteTo(raw, nil, authed.peer); err != nil {
				fmt.Printf("Failed to send packet: %s \n", err.Error())
			}
		} else {
			fmt.Println("Failed to encrypt packet")
			continue
		}

	}
}

func (p *Port) handleSRTP(srcString string, b BufferTransportGenerator, certPair *dtls.CertPair, buffer []byte, bufferSize int) {
	if len(buffer) > 4 {
		var rtcpPacketType uint8

		r := bytes.NewReader([]byte{buffer[1]})
		if err := binary.Read(r, binary.BigEndian, &rtcpPacketType); err != nil {
			fmt.Println("Failed to check packet for RTCP")
			return
		}

		if rtcpPacketType >= 192 && rtcpPacketType <= 223 {
			fmt.Println("Discarding RTCP packet TODO")
			return
		}
	}

	// Make copy of packet
	// buffer[:n] can't be modified outside of network loop
	rawPacket := make([]byte, bufferSize)
	copy(rawPacket, buffer[:bufferSize])

	packet := &rtp.Packet{}
	if err := packet.Unmarshal(rawPacket); err != nil {
		fmt.Println("Failed to unmarshal RTP packet")
		return
	}

	contextMapKey := srcString + ":" + fmt.Sprint(packet.SSRC)
	p.srtpContextsLock.Lock()
	srtpContext, ok := p.srtpContexts[contextMapKey]
	if !ok {
		var err error
		srtpContext, err = srtp.CreateContext([]byte(certPair.ServerWriteKey[0:16]), []byte(certPair.ServerWriteKey[16:]), certPair.Profile, packet.SSRC)
		if err != nil {
			fmt.Println("Failed to build SRTP context")
			return
		}

		p.srtpContexts[contextMapKey] = srtpContext
	}
	p.srtpContextsLock.Unlock()

	if ok := srtpContext.DecryptPacket(packet); !ok {
		fmt.Println("Failed to decrypt packet")
		return
	}

	bufferTransport := p.bufferTransports[packet.SSRC]
	if bufferTransport == nil {
		bufferTransport = b(packet.SSRC)
		if bufferTransport == nil {
			fmt.Println("Failed to generate buffer transport, onTrack should be defined")
			return
		}
		p.bufferTransports[packet.SSRC] = bufferTransport
	}
	bufferTransport <- packet
}

func (p *Port) networkLoop(srcString string, remoteKey []byte, tlscfg *dtls.TLSCfg, b BufferTransportGenerator) {
	const MTU = 8192
	buffer := make([]byte, MTU)

	var certPair *dtls.CertPair
	for {
		n, _, rawDstAddr, err := p.conn.ReadFrom(buffer)
		if err != nil {
			fmt.Printf("Failed to read packet: %s \n", err.Error())
			continue
		}

		d, haveHandshaked := p.dtlsStates[rawDstAddr.String()]
		if haveHandshaked {
			if handled, tmpCertPair := d.MaybeHandleDTLSPacket(buffer, n); handled {

				if tmpCertPair != nil {
					certPair = tmpCertPair
					p.authedConnections = append(p.authedConnections, &authedConnection{
						pair: certPair,
						peer: rawDstAddr,
					})
				}
				continue
			}
		}

		if packetType, err := stun.GetPacketType(buffer[:n]); err == nil && packetType == stun.PacketTypeSTUN {
			if m, err := stun.NewMessage(buffer[:n]); err == nil && m.Class == stun.ClassRequest && m.Method == stun.MethodBinding {
				dstAddr := &stun.TransportAddr{IP: rawDstAddr.(*net.UDPAddr).IP, Port: rawDstAddr.(*net.UDPAddr).Port}
				if err := stun.BuildAndSend(p.conn, dstAddr, stun.ClassSuccessResponse, stun.MethodBinding, m.TransactionID,
					&stun.XorMappedAddress{
						XorAddress: stun.XorAddress{
							IP:   dstAddr.IP,
							Port: dstAddr.Port,
						},
					},
					&stun.MessageIntegrity{
						Key: remoteKey,
					},
					&stun.Fingerprint{},
				); err != nil {
					fmt.Println(err)
				}
			}
		} else {
			if certPair == nil {
				fmt.Println("SRTP packet, but unable to handle DTLS handshake has not completed")
				continue
			}
			p.handleSRTP(srcString, b, certPair, buffer, n)
		}

		if !haveHandshaked {
			d, err := dtls.NewState(tlscfg, true, srcString, rawDstAddr.String())
			if err != nil {
				fmt.Println(err)
				continue
			}

			d.DoHandshake()
			p.dtlsStates[rawDstAddr.String()] = d
		}
	}

}
