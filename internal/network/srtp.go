package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/rtcp"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
)

const (
	srtpMasterKeyLen     = 16
	srtpMasterKeySaltLen = 14
)

// TODO: Migrate to srtp.Conn

// CreateContextSRTP takes the exported keying material from DTLS and creates Client/Server contexts
func (m *Manager) CreateContextSRTP(keyingMaterial []byte) error {
	offset := 0

	clientWriteKey := append([]byte{}, keyingMaterial[offset:offset+srtpMasterKeyLen]...)
	offset += srtpMasterKeyLen

	serverWriteKey := append([]byte{}, keyingMaterial[offset:offset+srtpMasterKeyLen]...)
	offset += srtpMasterKeyLen

	clientWriteKey = append(clientWriteKey, keyingMaterial[offset:offset+srtpMasterKeySaltLen]...)
	offset += srtpMasterKeySaltLen

	serverWriteKey = append(serverWriteKey, keyingMaterial[offset:offset+srtpMasterKeySaltLen]...)

	var err error
	m.srtpInboundContextLock.Lock()
	m.srtpInboundContext, err = srtp.CreateContext(serverWriteKey[0:16], serverWriteKey[16:] /* Profile */, "")
	m.srtpInboundContextLock.Unlock()
	if err != nil {
		return errors.New("failed to build inbound SRTP context")
	}

	m.srtpOutboundContextLock.Lock()
	m.srtpOutboundContext, err = srtp.CreateContext(clientWriteKey[0:16], clientWriteKey[16:] /* Profile */, "")
	m.srtpOutboundContextLock.Unlock()
	if err != nil {
		return errors.New("failed to build outbound SRTP context")
	}

	return nil
}

func handleRTCP(getBufferTransports func(uint32) *TransportPair, buffer []byte) {
	//decrypted packets can also be compound packets, so we have to nest our reader loop here.
	compoundPacket := rtcp.NewReader(bytes.NewReader(buffer))
	for {
		header, rawrtcp, err := compoundPacket.ReadPacket()

		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println(err)
			return
		}

		var report rtcp.Packet
		report, header, err = rtcp.Unmarshal(rawrtcp)
		if err != nil {
			fmt.Println(err)
			return
		}

		f := func(ssrc uint32) {
			bufferTransport := getBufferTransports(ssrc)
			if bufferTransport != nil && bufferTransport.RTCP != nil {
				select {
				case bufferTransport.RTCP <- report:
				default:
				}
			}
		}

		switch header.Type {
		case rtcp.TypeSenderReport:
			for _, ssrc := range report.(*rtcp.SenderReport).Reports {
				f(ssrc.SSRC)
			}
		case rtcp.TypeReceiverReport:
			for _, ssrc := range report.(*rtcp.ReceiverReport).Reports {
				f(ssrc.SSRC)
			}
		case rtcp.TypeSourceDescription:
			for _, ssrc := range report.(*rtcp.SourceDescription).Chunks {
				f(ssrc.Source)
			}
		case rtcp.TypeGoodbye:
			for _, ssrc := range report.(*rtcp.Goodbye).Sources {
				f(ssrc)
			}
		case rtcp.TypeTransportSpecificFeedback:
			f(report.(*rtcp.RapidResynchronizationRequest).MediaSSRC)
		case rtcp.TypePayloadSpecificFeedback:
			f(report.(*rtcp.PictureLossIndication).MediaSSRC)
		}
	}
}

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

			handleRTCP(m.getBufferTransports, decrypted)
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

	bufferTransport := m.getOrCreateBufferTransports(packet.SSRC, packet.PayloadType)
	if bufferTransport != nil && bufferTransport.RTP != nil {
		select {
		case bufferTransport.RTP <- packet:
		default:
		}
	}

}
