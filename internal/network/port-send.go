package network

import (
	"fmt"
	"net"

	"github.com/pions/webrtc/pkg/rtp"
)

func (p *port) sendRTP(packet *rtp.Packet, dst net.Addr) {
	p.m.srtpOutboundContextLock.Lock()
	defer p.m.srtpOutboundContextLock.Unlock()
	if p.m.srtpOutboundContext == nil {
		// TODO log-level
		// fmt.Printf("Tried to send RTP packet but no SRTP Context to handle it \n")
		return
	}

	if ok := p.m.srtpOutboundContext.EncryptRTP(packet); ok {
		raw, err := packet.Marshal()
		if err != nil {
			fmt.Printf("Failed to marshal packet: %s \n", err.Error())
		}
		if _, err := p.conn.WriteTo(raw, nil, dst); err != nil {
			fmt.Printf("Failed to send packet: %s \n", err.Error())
		}
	} else {
		fmt.Println("Failed to encrypt packet")
	}
}

func (p *port) sendSCTP(buf []byte, dst fmt.Stringer) {
	_, err := p.m.dtlsState.Send(buf, p.listeningAddr.String(), dst.String())
	if err != nil {
		fmt.Println(err)
	}
}

func (p *port) sendRTCP(buf []byte, dst net.Addr) {
	p.m.srtpOutboundContextLock.Lock()
	defer p.m.srtpOutboundContextLock.Unlock()
	if p.m.srtpOutboundContext == nil {
		fmt.Printf("Tried to send RTCP packet but no SRTP Context to handle it \n")
		return
	}

	encrypted, err := p.m.srtpOutboundContext.EncryptRTCP(buf)
	if err != nil {
		fmt.Println(err)
		return
	}

	if _, err := p.conn.WriteTo(encrypted, nil, dst); err != nil {
		fmt.Printf("Failed to send packet: %s \n", err.Error())
	}
}
