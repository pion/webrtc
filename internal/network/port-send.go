package network

import (
	"fmt"
	"net"

	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/rtp"
)

func (p *port) sendRTP(packet *rtp.Packet) {
	p.m.certPairLock.RLock()
	defer p.m.certPairLock.RUnlock()
	if p.m.certPair == nil {
		fmt.Printf("Tried to send SRTP packet but no DTLS state to handle it %v \n", p.m.certPair)
		return
	}

	var peer *net.UDPAddr
	p.seenPeersLock.RLock()
	for _, p := range p.seenPeers {
		peer = p
		break
	}
	p.seenPeersLock.RUnlock()

	if peer == nil {
		fmt.Printf("No peers to send to, ICE state is %s \n", p.iceState.String())
		return
	}

	contextMapKey := peer.String() + ":" + fmt.Sprint(packet.SSRC)
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
		if _, err := p.conn.WriteTo(raw, nil, peer); err != nil {
			fmt.Printf("Failed to send packet: %s \n", err.Error())
		}
	} else {
		fmt.Println("Failed to encrypt packet")
	}
}

func (p *port) sendSCTP(buf []byte) {
	var peer *net.UDPAddr

	p.seenPeersLock.RLock()
	for _, p := range p.seenPeers {
		peer = p
		break
	}
	p.seenPeersLock.RUnlock()

	if peer == nil {
		fmt.Printf("No peers to send to, ICE state is %s \n", p.iceState.String())
		return
	}

	p.m.dtlsState.Send(buf, p.listeningAddr.String(), peer.String())
}
