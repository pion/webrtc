package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/datachannel"
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

func (p *Port) handleSRTP(b BufferTransportGenerator, buffer []byte) {
	if p.certPair == nil {
		fmt.Printf("Got SRTP packet but no DTLS state to handle it %v \n", p.certPair)
		return
	}
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
		srtpContext, err = srtp.CreateContext([]byte(p.certPair.ServerWriteKey[0:16]), []byte(p.certPair.ServerWriteKey[16:]), p.certPair.Profile, packet.SSRC)
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

func (p *Port) handleSCTP(raw []byte, a *sctp.Association) {
	pkt := &sctp.Packet{}
	if err := pkt.Unmarshal(raw); err != nil {
		fmt.Println(errors.Wrap(err, "Failed to Unmarshal SCTP packet"))
		return
	}

	if err := a.PushPacket(pkt); err != nil {
		fmt.Println(errors.Wrap(err, "Failed to push SCTP packet"))
	}
}

func (p *Port) handleDTLS(raw []byte, srcAddr *net.UDPAddr) {
	dtlsState := p.dtlsStates[srcAddr.String()]
	association := p.sctpAssocations[srcAddr.String()]
	if dtlsState == nil || association == nil {
		fmt.Printf("Got DTLS packet but no DTLS/SCTP state to handle it %v %v \n", dtlsState, association)
	}

	if decrypted := dtlsState.HandleDTLSPacket(raw); len(decrypted) > 0 {
		p.handleSCTP(decrypted, association)
	}

	if certPair := dtlsState.GetCertPair(); certPair != nil && p.certPair == nil {
		p.certPair = certPair
		if p.certPair != nil {
			p.authedConnections = append(p.authedConnections, &authedConnection{
				pair: p.certPair,
				peer: srcAddr,
			})
		}
	}
}

const iceTimeout = time.Second * 10
const receiveMTU = 8192

func (p *Port) networkLoop(remoteKey []byte, tlscfg *dtls.TLSCfg, b BufferTransportGenerator, iceNotifier ICENotifier, dceh DataChannelEventHandler) {
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
				for _, a := range p.sctpAssocations {
					a.Close()
				}
				for _, d := range p.dtlsStates {
					d.Close()
				}
				dtls.RemoveListener(p.ListeningAddr.String())
				return
			}

			if len(in.buffer) == 0 {
				fmt.Println("Inbound buffer is not long enough to demux")
				continue
			}

			// https://tools.ietf.org/html/rfc5764#page-14
			if 127 < in.buffer[0] && in.buffer[0] < 192 {
				p.handleSRTP(b, in.buffer)
			} else if 19 < in.buffer[0] && in.buffer[0] < 64 {
				p.handleDTLS(in.buffer, in.srcAddr)
			} else if in.buffer[0] < 2 {
				p.handleICE(in, remoteKey, iceTimer, iceNotifier)
			}

			if _, ok := p.dtlsStates[in.srcAddr.String()]; !ok {
				d, err := dtls.NewState(tlscfg, true, p.ListeningAddr.String(), in.srcAddr.String())
				if err != nil {
					fmt.Println(err)
					continue
				}

				d.DoHandshake()
				p.dtlsStates[in.srcAddr.String()] = d
				p.sctpAssocations[in.srcAddr.String()] = sctp.NewAssocation(func(pkt *sctp.Packet) {
					raw, err := pkt.Marshal()
					if err != nil {
						fmt.Println(errors.Wrap(err, "Failed to Marshal SCTP packet"))
						return
					}
					d.Send(raw)
				}, func(data []byte, streamIdentifier uint16) {
					msg, err := datachannel.Parse(data)
					if err != nil {
						fmt.Println(errors.Wrap(err, "Failed to parse DataChannel packet"))
						return
					}
					switch m := msg.(type) {
					case *datachannel.ChannelOpen:
						dceh(&DataChannelCreated{streamIdentifier: streamIdentifier, Label: string(m.Label)})
					case *datachannel.Data:
						dceh(&DataChannelMessage{streamIdentifier: streamIdentifier, Body: m.Data})
					default:
						fmt.Println("Unhandled DataChannel message", m)
					}
				})
			}
		}
	}
}
