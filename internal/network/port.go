package network

import (
	"fmt"
	"net"
	"strconv"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/rtp"
	"golang.org/x/net/ipv4"
)

// Port represents a UDP listener that handles incoming/outgoing traffic
type Port struct {
	ListeningAddr *stun.TransportAddr

	conn                     *ipv4.PacketConn
	srcString                string
	remoteKey                []byte
	tlscfg                   *dtls.TLSCfg
	bufferTransportGenerator BufferTransportGenerator
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
		ListeningAddr: addr,

		conn:      conn,
		srcString: srcString,
		remoteKey: remoteKey,
		tlscfg:    tlscfg,
		bufferTransportGenerator: b,
	}

	go p.packetHandler()
	return p, nil
}

func (p *Port) Stop() {
}

func (p *Port) packetHandler() {
	const MTU = 8192
	buffer := make([]byte, MTU)

	dtlsStates := make(map[string]*dtls.State)
	bufferTransports := make(map[uint32]chan<- *rtp.Packet)

	// TODO multiple SRTP Contexts
	// https://tools.ietf.org/html/rfc3711#section-3.2.3
	// A cryptographic context SHALL be uniquely identified by the triplet
	//  <SSRC, destination network address, destination transport port number>
	var srtpContext *srtp.Context
	for {
		n, _, rawDstAddr, err := p.conn.ReadFrom(buffer)
		if err != nil {
			fmt.Printf("Failed to read packet: %s \n", err.Error())
			continue
		}

		d, haveHandshaked := dtlsStates[rawDstAddr.String()]
		if haveHandshaked {
			if handled, certPair := d.MaybeHandleDTLSPacket(buffer, n); handled {
				if certPair != nil {
					srtpContext, err = srtp.CreateContext([]byte(certPair.ServerWriteKey[0:16]), []byte(certPair.ServerWriteKey[16:]), certPair.Profile)
					if err != nil {
						fmt.Println(err)
						continue
					}
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
						Key: p.remoteKey,
					},
					&stun.Fingerprint{},
				); err != nil {
					fmt.Println(err)
				}
			}
		} else if srtpContext != nil {
			// Make copy of packet
			// buffer[:n] can't be modified outside of network loop
			rawPacket := make([]byte, n)
			copy(rawPacket, buffer[:n])

			packet := &rtp.Packet{}
			if err := packet.Unmarshal(rawPacket); err != nil {
				fmt.Println("Failed to unmarshal RTP packet")
				continue
			}

			if ok := srtpContext.DecryptPacket(packet); !ok {
				fmt.Println("Failed to decrypt packet")
				continue
			}

			bufferTransport := bufferTransports[packet.SSRC]
			if bufferTransport == nil {
				bufferTransport = p.bufferTransportGenerator(packet.SSRC)
				if bufferTransport == nil {
					fmt.Println("Failed to generate buffer transport, onTrack should be defined")
					continue
				}
				bufferTransports[packet.SSRC] = bufferTransport
			}
			bufferTransport <- packet
		} else {
			fmt.Println("SRTP packet, but no srtpSession")
		}

		if !haveHandshaked {
			d, err := dtls.NewState(p.tlscfg, true, p.srcString, rawDstAddr.String())
			if err != nil {
				fmt.Println(err)
				continue
			}

			d.DoHandshake()
			dtlsStates[rawDstAddr.String()] = d
		}
	}

}
