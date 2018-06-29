package network

import (
	"fmt"

	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/rtp"
)

// Send sends a *rtp.Packet if we have a connected peer
func (p *Port) Send(packet *rtp.Packet) {
	var err error

	for _, authed := range p.authedConnections {

		contextMapKey := authed.peer.String() + ":" + fmt.Sprint(packet.SSRC)
		p.srtpContextsLock.Lock()
		srtpContext, ok := p.srtpContexts[contextMapKey]
		if !ok {
			srtpContext, err = srtp.CreateContext([]byte(authed.pair.ClientWriteKey[0:16]), []byte(authed.pair.ClientWriteKey[16:]), authed.pair.Profile, 2581832418)
			if err != nil {
				fmt.Println("Failed to build SRTP context")
				continue
			}

			p.srtpContexts[contextMapKey] = srtpContext
		}
		p.srtpContextsLock.Unlock()

		if ok := srtpContext.EncryptPacket(packet); ok {
			raw, err := packet.Marshal()
			if err != nil {
				fmt.Printf("Failed to marshal packet: %s \n", err.Error())
			}
			if _, err := p.conn.WriteTo(raw, nil, authed.peer); err != nil {
				fmt.Printf("Failed to send packet: %s \n", err.Error())
			}
		} else {
			fmt.Println("Failed to encrypt packet")
			continue
		}

	}
}
