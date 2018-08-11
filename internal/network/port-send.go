package network

import (
	"fmt"
	"net"

	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/rtp"
)

func (p *port) sendRTP(packet *rtp.Packet, dst net.Addr) {
	p.m.certPairLock.RLock()
	defer p.m.certPairLock.RUnlock()
	if p.m.certPair == nil {
		fmt.Println("Tried to send SRTP packet but no DTLS state to handle it")
		return
	}

	contextMapKey := dst.String() + ":" + fmt.Sprint(packet.SSRC)
	p.m.srtpContextsLock.Lock()
	srtpContext, ok := p.m.srtpContexts[contextMapKey]
	if !ok {
		var err error
		srtpContext, err = srtp.CreateContext([]byte(p.m.certPair.ClientWriteKey[0:16]), []byte(p.m.certPair.ClientWriteKey[16:]), p.m.certPair.Profile, packet.SSRC)
		if err != nil {
			fmt.Println("Failed to build SRTP context")
			return
		}

		p.m.srtpContexts[contextMapKey] = srtpContext
	}
	p.m.srtpContextsLock.Unlock()

	if ok := srtpContext.EncryptPacket(packet); ok {
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
