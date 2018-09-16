package network

import (
	"fmt"
	"net"

	"github.com/pions/webrtc/pkg/rtp"
)

func (p *port) sendRTP(packet *rtp.Packet, dst net.Addr) {
	p.m.srtpInboundContextLock.Lock()
	defer p.m.srtpInboundContextLock.Unlock()
	if p.m.srtpInboundContext == nil {
		fmt.Printf("Tried to send RTP packet but no SRTP Context to handle it \n")
		return
	}

	if ok := p.m.srtpOutboundContext.EncryptPacket(packet); ok {
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

func (p *port) sendICE(buf []byte, dst net.Addr) {
	if _, err := p.conn.WriteTo(buf, nil, dst); err != nil {
		fmt.Printf("Failed to send packet: %s \n", err.Error())
	}
}

func (p *port) sendSCTP(buf []byte, dst fmt.Stringer) {
	_, err := p.m.dtlsState.Send(buf, p.listeningAddr.String(), dst.String())
	if err != nil {
		fmt.Println(err)
	}
}
