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
	"github.com/pkg/errors"
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

func (p *Port) handleSCTP(raw []byte, dtlsState *dtls.State) {
	s := &sctp.Packet{}
	if err := s.Unmarshal(raw); err != nil {
		fmt.Println(errors.Wrap(err, "Failed to Unmarshal SCTP packet"))
		return
	}

	for _, c := range s.Chunks {
		switch v := c.(type) {
		case *sctp.Init:
			fmt.Println("Got an init chunk")
			dtlsState.Send([]byte{0x00, 0x01, 0x01, 0x02, 0x03, 0x05, 0x08})
		default:
			fmt.Println("Unhandled chunk!", v)
		}
	}
}

func (p *Port) handleDTLS(raw []byte, srcAddr *net.UDPAddr, certPair *dtls.CertPair) bool {
	if len(raw) < 0 || (raw[0] < 19 || raw[0] > 65) {
		return false
	}

	dtlsState := p.dtlsStates[srcAddr.String()]
	if dtlsState == nil {
		return true
	}

	if decrypted := dtlsState.HandleDTLSPacket(raw); len(decrypted) > 0 {
		p.handleSCTP(decrypted, dtlsState)
	}

	if certPair == nil {
		certPair = dtlsState.GetCertPair()
		if certPair != nil {
			p.authedConnections = append(p.authedConnections, &authedConnection{
				pair: certPair,
				peer: srcAddr,
			})
		}
	}

	return true
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
				dtls.RemoveListener(p.ListeningAddr.String())
				for _, d := range p.dtlsStates {
					d.Close()
				}
				break
			}

			if p.handleDTLS(in.buffer, in.srcAddr, certPair) {
				continue
			}

			if packetType, err := stun.GetPacketType(in.buffer); err == nil && packetType == stun.PacketTypeSTUN {
				p.handleICE(in, remoteKey, iceTimer, iceNotifier)
			} else if certPair == nil {
				fmt.Println("SRTP packet, but unable to handle DTLS handshake has not completed")
			} else {
				p.handleSRTP(b, certPair, in.buffer)
			}

			if _, ok := p.dtlsStates[in.srcAddr.String()]; !ok {
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
}
