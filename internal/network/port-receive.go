package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/sctp"
	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtp"
)

type incomingPacket struct {
	srcAddr *net.UDPAddr
	buffer  []byte
}

func (p *Port) handleSRTP(b BufferTransportGenerator, certPair *dtls.CertPair, buffer []byte) {
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

	packet := &rtp.Packet{}
	if err := packet.Unmarshal(buffer); err != nil {
		fmt.Println("Failed to unmarshal RTP packet")
		return
	}

	contextMapKey := p.ListeningAddr.String() + ":" + fmt.Sprint(packet.SSRC)
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
		bufferTransport = b(packet.SSRC, packet.PayloadType)
		if bufferTransport == nil {
			return
		}
		p.bufferTransports[packet.SSRC] = bufferTransport
	}

	select {
	case bufferTransport <- packet:
	default:
	}

}

func (p *Port) handleICE(in *incomingPacket, remoteKey []byte, iceTimer *time.Timer, iceNotifier ICENotifier) {
	if m, err := stun.NewMessage(in.buffer); err == nil && m.Class == stun.ClassRequest && m.Method == stun.MethodBinding {
		dstAddr := &stun.TransportAddr{IP: in.srcAddr.IP, Port: in.srcAddr.Port}
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
		} else {
			p.ICEState = ice.ConnectionStateCompleted
			iceTimer.Reset(iceTimeout)
			iceNotifier(p)
		}
	}
}

const iceTimeout = time.Second * 10
const receiveMTU = 8192

func (p *Port) networkLoop(remoteKey []byte, tlscfg *dtls.TLSCfg, b BufferTransportGenerator, iceNotifier ICENotifier) {
	incomingPackets := make(chan *incomingPacket, 15)
	go func() {
		buffer := make([]byte, receiveMTU)
		for {
			n, _, srcAddr, err := p.conn.ReadFrom(buffer)
			if err != nil {
				close(incomingPackets)
				break
			}

			bufferCopy := make([]byte, n)
			copy(bufferCopy, buffer[:n])

			select {
			case incomingPackets <- &incomingPacket{buffer: bufferCopy, srcAddr: srcAddr.(*net.UDPAddr)}:
			default:
			}
		}
	}()

	var certPair *dtls.CertPair
	// Never timeout originally, only start timer after we get an ICE ping
	iceTimer := time.NewTimer(time.Hour * 8760)
	for {
		select {
		case <-iceTimer.C:
			p.ICEState = ice.ConnectionStateFailed
			iceNotifier(p)
		case in, inValid := <-incomingPackets:
			if !inValid {
				// incomingPackets channel has closed, this port is finished processing
				return
			}

			dtlsState := p.dtlsStates[in.srcAddr.String()]
			if dtlsState != nil && len(in.buffer) > 0 && in.buffer[0] >= 20 && in.buffer[0] <= 64 {
				decrypted := dtlsState.HandleDTLSPacket(in.buffer)
				if len(decrypted) > 0 {
					sctp.HandlePacket(decrypted)
				}

				if certPair == nil {
					certPair = dtlsState.GetCertPair()
					if certPair != nil {
						p.authedConnections = append(p.authedConnections, &authedConnection{
							pair: certPair,
							peer: in.srcAddr,
						})
					}
				}
				continue
			}

			if packetType, err := stun.GetPacketType(in.buffer); err == nil && packetType == stun.PacketTypeSTUN {
				p.handleICE(in, remoteKey, iceTimer, iceNotifier)
			} else if certPair == nil {
				fmt.Println("SRTP packet, but unable to handle DTLS handshake has not completed")
			} else {
				p.handleSRTP(b, certPair, in.buffer)
			}

			if dtlsState == nil {
				d, err := dtls.NewState(tlscfg, true, p.ListeningAddr.String(), in.srcAddr.String())
				if err != nil {
					fmt.Println(err)
					continue
				}

				d.DoHandshake()
				p.dtlsStates[in.srcAddr.String()] = d

			}
		}
	}
	dtls.RemoveListener(p.ListeningAddr.String())
	for _, d := range p.dtlsStates {
		d.Close()
	}
}
