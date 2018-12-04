package rtcp

import (
	"encoding/binary"
	"fmt"
	"math"
)

// PacketBitmap shouldn't be used like a normal integral,
// so it's type is masked here. Access it with PacketList().
type PacketBitmap uint16

// NackPair is a wire-representation of a collection of
// Lost RTP packets
type NackPair struct {
	// ID of lost packets
	PacketID uint16

	// Bitmask of following lost packets
	LostPackets PacketBitmap
}

// The TransportLayerNack packet informs the encoder about the loss of a transport packet
// IETF RFC 4585, Section 6.2.1
// https://tools.ietf.org/html/rfc4585#section-6.2.1
type TransportLayerNack struct {
	// SSRC of sender
	SenderSSRC uint32

	// SSRC of the media source
	MediaSSRC uint32

	Nacks []NackPair
}

// PacketList returns a list of Nack'd packets that's referenced by a NackPair
func (n *NackPair) PacketList() []uint16 {
	out := make([]uint16, 1, 17)
	out[0] = n.PacketID
	b := n.LostPackets
	for i := uint16(0); b != 0; i++ {
		if (b & (1 << i)) != 0 {
			b &^= (1 << i)
			out = append(out, n.PacketID+i+1)
		}
	}
	return out
}

const (
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
		binary.BigEndian.PutUint16(rawPacket[nackOffset+(4*i)+2:], uint16(p.Nacks[i].LostPackets))
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
	if len(rawPacket) < (headerLength + ssrcLength) {
		return errPacketTooShort
	}

	var h Header
	if err := h.Unmarshal(rawPacket); err != nil {
		return err
	}

	if len(rawPacket) < (headerLength + int(4*h.Length)) {
		return errPacketTooShort
	}

	if h.Type != TypeTransportSpecificFeedback || h.Count != FormatTLN {
		return errWrongType
	}

	p.SenderSSRC = binary.BigEndian.Uint32(rawPacket[headerLength:])
	p.MediaSSRC = binary.BigEndian.Uint32(rawPacket[headerLength+ssrcLength:])
	for i := headerLength + nackOffset; i < (headerLength + int(h.Length*4)); i += 4 {
		p.Nacks = append(p.Nacks, NackPair{
			binary.BigEndian.Uint16(rawPacket[i:]),
			PacketBitmap(binary.BigEndian.Uint16(rawPacket[i+2:])),
		})
	}
	return nil
}

func (p *TransportLayerNack) len() int {
	return headerLength + nackOffset + (len(p.Nacks) * 4)
}

// Header returns the Header associated with this packet.
func (p *TransportLayerNack) Header() Header {
	return Header{
		Count:  FormatTLN,
		Type:   TypeTransportSpecificFeedback,
		Length: uint16((p.len() / 4) - 1),
	}
}

func (p *TransportLayerNack) String() string {
	o := "Packets Lost:\n"
	for _, n := range p.Nacks {
		for _, m := range n.PacketList() {
			o += fmt.Sprintf("\t%d\n", m)
		}
	}
	return o
}

// DestinationSSRC returns an array of SSRC values that this packet refers to.
func (p *TransportLayerNack) DestinationSSRC() []uint32 {
	return []uint32{p.MediaSSRC}
}
