package rtcp

import (
	"encoding/binary"
	"fmt"
	"math"
)

// The TransportLayerNack packet informs the encoder about the loss of a transport packet
type TransportLayerNack struct {
	// SSRC of sender
	SenderSSRC uint32

	// SSRC of the media source
	MediaSSRC uint32

	Nacks []struct {
		// ID of lost packets
		PacketID uint16

		// Bitmask of following lost packets
		BLP uint16
	}
}

const (
	tlnFMT     = 1
	tlnLength  = 2
	nackOffset = 8
)

// Marshal encodes the TransportLayerNack in binary
func (p TransportLayerNack) Marshal() ([]byte, error) {

	if len(p.Nacks)+tlnLength > math.MaxUint8 {
		return nil, errTooManyReports
	}

	rawPacket := make([]byte, nackOffset+(len(p.Nacks)*4))
	binary.BigEndian.PutUint32(rawPacket, p.SenderSSRC)
	binary.BigEndian.PutUint32(rawPacket[4:], p.MediaSSRC)
	for i := 0; i < len(p.Nacks); i++ {
		binary.BigEndian.PutUint16(rawPacket[nackOffset+(4*i):], p.Nacks[i].PacketID)
		binary.BigEndian.PutUint16(rawPacket[nackOffset+(4*i)+2:], p.Nacks[i].BLP)
	}
	h := p.Header()
	hData, err := h.Marshal()
	if err != nil {
		return nil, err
	}

	return append(hData, rawPacket...), nil
}

// Unmarshal decodes the TransportLayerNack from binary
func (p *TransportLayerNack) Unmarshal(rawPacket []byte) error {
	var h Header
	if err := h.Unmarshal(rawPacket); err != nil {
		return err
	}

	if len(rawPacket) < (headerLength + int(4*h.Length)) {
		return errPacketTooShort
	}

	if h.Type != TypeTransportSpecificFeedback || h.Count != tlnFMT {
		return errWrongType
	}

	p.SenderSSRC = binary.BigEndian.Uint32(rawPacket[headerLength:])
	p.MediaSSRC = binary.BigEndian.Uint32(rawPacket[headerLength+ssrcLength:])
	for i := headerLength + nackOffset; i < (headerLength + int(h.Length*4)); i += 4 {
		p.Nacks = append(p.Nacks, struct {
			PacketID uint16
			BLP      uint16
		}{
			binary.BigEndian.Uint16(rawPacket[i:]),
			binary.BigEndian.Uint16(rawPacket[i+2:])})
	}
	return nil
}

func (p *TransportLayerNack) len() int {
	return headerLength + nackOffset + (len(p.Nacks) * 4)
}

// Header returns the Header associated with this packet.
func (p *TransportLayerNack) Header() Header {
	return Header{
		Count:  tlnFMT,
		Type:   TypeTransportSpecificFeedback,
		Length: uint16((p.len() / 4) - 1),
	}
}

func (p *TransportLayerNack) String() string {
	o := "Packets Lost:\n"
	for _, n := range p.Nacks {
		b := n.BLP
		o += fmt.Sprintf("\t%d\n", n.PacketID)
		for i := uint16(0); b != 0; i++ {
			if (b & (1 << i)) != 0 {
				b &^= (1 << i)
				o += fmt.Sprintf("\t%d\n", n.PacketID+i+1)
			}
		}
	}
	return o
}

// DestinationSSRC returns an array of SSRC values that this packet refers to.
func (p *TransportLayerNack) DestinationSSRC() []uint32 {
	return []uint32{p.MediaSSRC}
}
