package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/sctp"
	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
)

type incomingPacket struct {
	srcAddr *net.UDPAddr
	buffer  []byte
}

func (p *port) handleSRTP(buffer []byte) {
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

	p.m.srtpInboundContextLock.Lock()
	defer p.m.srtpInboundContextLock.Unlock()
	if p.m.srtpInboundContext == nil {
		fmt.Printf("Got RTP packet but no SRTP Context to handle it \n")
		return
	}

	if ok := p.m.srtpInboundContext.DecryptPacket(packet); !ok {
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

func (p *port) handleSCTP(raw []byte, a *sctp.Association) {
	p.m.sctpAssociation.Lock()
	defer p.m.sctpAssociation.Unlock()

	if err := a.HandleInbound(raw); err != nil {
		fmt.Println(errors.Wrap(err, "Failed to push SCTP packet"))
	}
}

func (p *port) handleDTLS(raw []byte, srcAddr string) {
	decrypted, err := p.m.dtlsState.HandleDTLSPacket(raw, p.listeningAddr.String(), srcAddr)
	if err != nil {
		fmt.Println(err)
		return
	}

	if len(decrypted) > 0 {
		p.handleSCTP(decrypted, p.m.sctpAssociation)
	}

	p.m.certPairLock.Lock()
	if certPair := p.m.dtlsState.GetCertPair(); certPair != nil && p.m.certPair == nil {
		var err error
		p.m.certPair = certPair

		p.m.srtpInboundContextLock.Lock()
		p.m.srtpInboundContext, err = srtp.CreateContext(p.m.certPair.ServerWriteKey[0:16], p.m.certPair.ServerWriteKey[16:], p.m.certPair.Profile)
		p.m.srtpInboundContextLock.Unlock()
		if err != nil {
			fmt.Println("Failed to build SRTP context, this is fatal")
			return
		}

		p.m.srtpOutboundContextLock.Lock()
		p.m.srtpOutboundContext, err = srtp.CreateContext(p.m.certPair.ClientWriteKey[0:16], p.m.certPair.ClientWriteKey[16:], p.m.certPair.Profile)
		p.m.srtpOutboundContextLock.Unlock()
		if err != nil {
			fmt.Println("Failed to build SRTP context, this is fatal")
			return
		}

	}
	p.m.certPairLock.Unlock()

}

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

	for {
		in, socketOpen := <-incomingPackets
		if !socketOpen {
			// incomingPackets channel has closed, this port is finished processing
			dtls.RemoveListener(p.listeningAddr.String())
			return
		}

		if len(in.buffer) == 0 {
			fmt.Println("Inbound buffer is not long enough to demux")
			continue
		}

		// https://tools.ietf.org/html/rfc5764#page-14
		if 127 < in.buffer[0] && in.buffer[0] < 192 {
			p.handleSRTP(in.buffer)
		} else if 19 < in.buffer[0] && in.buffer[0] < 64 {
			p.handleDTLS(in.buffer, in.srcAddr.String())
		} else if in.buffer[0] < 2 {
			p.m.IceAgent.HandleInbound(in.buffer, p.listeningAddr, in.srcAddr)
		}

		p.m.certPairLock.RLock()
		if !p.m.isOffer && p.m.certPair == nil {
			p.m.dtlsState.DoHandshake(p.listeningAddr.String(), in.srcAddr.String())
		}
		p.m.certPairLock.RUnlock()
	}
}
