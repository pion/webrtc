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

func (p *port) updateICEAndNotify(newState ice.ConnectionState) {
	p.iceStateLock.Lock()
	p.iceState = newState
	p.iceStateLock.Unlock()

	p.m.iceHandler(p.iceState)
}

func (p *port) handleSRTP(buffer []byte) {
	p.m.certPairLock.RLock()
	defer p.m.certPairLock.RUnlock()

	if p.m.certPair == nil {
		fmt.Printf("Got SRTP packet but no DTLS state to handle it %v \n", p.m.certPair)
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

	contextMapKey := p.listeningAddr.String() + ":" + fmt.Sprint(packet.SSRC)
	p.m.srtpContextsLock.Lock()
	srtpContext, ok := p.m.srtpContexts[contextMapKey]
	if !ok {
		var err error
		srtpContext, err = srtp.CreateContext([]byte(p.m.certPair.ServerWriteKey[0:16]), []byte(p.m.certPair.ServerWriteKey[16:]), p.m.certPair.Profile, packet.SSRC)
		if err != nil {
			fmt.Println("Failed to build SRTP context")
			return
		}

		p.m.srtpContexts[contextMapKey] = srtpContext
	}
	p.m.srtpContextsLock.Unlock()

	if ok := srtpContext.DecryptPacket(packet); !ok {
		fmt.Println("Failed to decrypt packet")
		return
	}

	bufferTransport := p.m.bufferTransports[packet.SSRC]
	if bufferTransport == nil {
		bufferTransport = p.m.bufferTransportGenerator(packet.SSRC, packet.PayloadType)
		if bufferTransport == nil {
			return
		}
		p.m.bufferTransports[packet.SSRC] = bufferTransport
	}

	select {
	case bufferTransport <- packet:
	default:
	}

}

func (p *port) handleICE(in *incomingPacket, iceTimer *time.Timer) {
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
				Key: p.m.icePwd,
			},
			&stun.Fingerprint{},
		); err != nil {
			fmt.Println(err)
		} else {
			p.updateICEAndNotify(ice.ConnectionStateCompleted)
			iceTimer.Reset(iceTimeout)
		}
	}
}

func (p *port) handleSCTP(raw []byte, a *sctp.Association) {
	p.m.sctpAssociation.Lock()
	defer p.m.sctpAssociation.Unlock()

	if err := a.HandleInbound(raw); err != nil {
		fmt.Println(errors.Wrap(err, "Failed to push SCTP packet"))
	}
}

func (p *port) handleDTLS(raw []byte, srcAddr string) {
	if decrypted := p.m.dtlsState.HandleDTLSPacket(raw, p.listeningAddr.String(), srcAddr); len(decrypted) > 0 {
		p.handleSCTP(decrypted, p.m.sctpAssociation)
	}

	p.m.certPairLock.Lock()
	if certPair := p.m.dtlsState.GetCertPair(); certPair != nil && p.m.certPair == nil {
		p.m.certPair = certPair
	}
	p.m.certPairLock.Unlock()

}

const iceTimeout = time.Second * 10
const noTimeout = time.Hour * 8760
const receiveMTU = 8192

func (p *port) networkLoop() {
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
	iceTimer := time.NewTimer(noTimeout)
	for {
		select {
		case <-iceTimer.C:
			p.updateICEAndNotify(ice.ConnectionStateDisconnected)
			iceTimer.Reset(noTimeout)
		case in, socketOpen := <-incomingPackets:
			if !socketOpen {
				// incomingPackets channel has closed, this port is finished processing
				dtls.RemoveListener(p.listeningAddr.String())
				return
			}

			if len(in.buffer) == 0 {
				fmt.Println("Inbound buffer is not long enough to demux")
				continue
			}

			// TODO these should age out, a candidate might not be good forever
			p.seenPeersLock.Lock()
			p.seenPeers[in.srcAddr.String()] = in.srcAddr
			p.seenPeersLock.Unlock()

			// https://tools.ietf.org/html/rfc5764#page-14
			if 127 < in.buffer[0] && in.buffer[0] < 192 {
				p.handleSRTP(in.buffer)
			} else if 19 < in.buffer[0] && in.buffer[0] < 64 {
				p.handleDTLS(in.buffer, in.srcAddr.String())
			} else if in.buffer[0] < 2 {
				p.handleICE(in, iceTimer)
			}

			p.m.certPairLock.RLock()
			if p.m.certPair == nil {
				p.m.dtlsState.DoHandshake(p.listeningAddr.String(), in.srcAddr.String())
			}
			p.m.certPairLock.RUnlock()

		}
	}
}
