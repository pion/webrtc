package network

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/pions/webrtc/internal/sctp"
	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
)

func (m *Manager) handleSRTP(buffer []byte) {
	m.srtpInboundContextLock.Lock()
	defer m.srtpInboundContextLock.Unlock()
	if m.srtpInboundContext == nil {
		fmt.Printf("Got RTP packet but no SRTP Context to handle it \n")
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
			decrypted, err := m.srtpInboundContext.DecryptRTCP(buffer)
			if err != nil {
				fmt.Println(err)
				fmt.Println(decrypted)
				return
			}
			return
		}
	}

	packet := &rtp.Packet{}
	if err := packet.Unmarshal(buffer); err != nil {
		fmt.Println("Failed to unmarshal RTP packet")
		return
	}

	if ok := m.srtpInboundContext.DecryptRTP(packet); !ok {
		fmt.Println("Failed to decrypt packet")
		return
	}

	bufferTransport := m.bufferTransports[packet.SSRC]
	if bufferTransport == nil {
		bufferTransport = m.bufferTransportGenerator(packet.SSRC, packet.PayloadType)
		if bufferTransport == nil {
			return
		}
		m.bufferTransports[packet.SSRC] = bufferTransport
	}

	select {
	case bufferTransport <- packet:
	default:
	}

}

func (m *Manager) handleSCTP(raw []byte, a *sctp.Association) {
	m.sctpAssociation.Lock()
	defer m.sctpAssociation.Unlock()

	if err := a.HandleInbound(raw); err != nil {
		fmt.Println(errors.Wrap(err, "Failed to push SCTP packet"))
	}
}

func (m *Manager) handleDTLS(raw []byte) {
	decrypted, err := m.dtlsState.HandleDTLSPacket(raw, "0.0.0.0", "0.0.0.0")
	if err != nil {
		fmt.Println(err)
		return
	}

	if len(decrypted) > 0 {
		m.handleSCTP(decrypted, m.sctpAssociation)
	}

	m.certPairLock.Lock()
	if certPair := m.dtlsState.GetCertPair(); certPair != nil && m.certPair == nil {
		var err error
		m.certPair = certPair

		m.srtpInboundContextLock.Lock()
		m.srtpInboundContext, err = srtp.CreateContext(m.certPair.ServerWriteKey[0:16],
			m.certPair.ServerWriteKey[16:],
			m.certPair.Profile)
		m.srtpInboundContextLock.Unlock()
		if err != nil {
			fmt.Println("Failed to build SRTP context, this is fatal")
			return
		}

		m.srtpOutboundContextLock.Lock()
		m.srtpOutboundContext, err = srtp.CreateContext(m.certPair.ClientWriteKey[0:16],
			m.certPair.ClientWriteKey[16:],
			m.certPair.Profile)
		m.srtpOutboundContextLock.Unlock()
		if err != nil {
			fmt.Println("Failed to build SRTP context, this is fatal")
			return
		}

	}
	m.certPairLock.Unlock()

}

const receiveMTU = 8192

func (m *Manager) networkLoop() {
	// TODO: Continue to phase out the networkLoop.

	buffer := make([]byte, receiveMTU)
	for {
		_, err := m.iceConn.Read(buffer)

		if err != nil {
			fmt.Println("NetworkLoop failed to read", err)
			return
		}

		// https://tools.ietf.org/html/rfc5764#page-14
		if 127 < buffer[0] && buffer[0] < 192 {
			m.handleSRTP(buffer)
		} else if 19 < buffer[0] && buffer[0] < 64 {
			m.handleDTLS(buffer)
		}
	}
}
