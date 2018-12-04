package rtcp

import (
	"encoding/binary"
	"fmt"
	"math"
)

// The SliceLossIndication packet informs the encoder about the loss of a picture slice
type SliceLossIndication struct {
	// SSRC of sender
	SenderSSRC uint32

	// SSRC of the media source
	MediaSSRC uint32

	SLI []struct {
		// ID of first lost slice
		First uint16

		// Number of lost slices
		Number uint16

		// ID of related picture
		Picture uint8
	}
}

const (
	sliFMT    = 2
	sliLength = 2
	sliOffset = 8
)

// Marshal encodes the SliceLossIndication in binary
func (p SliceLossIndication) Marshal() ([]byte, error) {

	if len(p.SLI)+sliLength > math.MaxUint8 {
		return nil, errTooManyReports
	}

	rawPacket := make([]byte, sliOffset+(len(p.SLI)*4))
	binary.BigEndian.PutUint32(rawPacket, p.SenderSSRC)
	binary.BigEndian.PutUint32(rawPacket[4:], p.MediaSSRC)
	for i := 0; i < len(p.SLI); i++ {
		sli := ((uint32(p.SLI[i].First) & 0x1FFF) << 19) |
			((uint32(p.SLI[i].Number) & 0x1FFF) << 6) |
			(uint32(p.SLI[i].Picture) & 0x3F)
		binary.BigEndian.PutUint32(rawPacket[sliOffset+(4*i):], sli)
	}
	h := Header{
		Count:  tlnFMT,
		Type:   TypeTransportSpecificFeedback,
		Length: uint16(tlnLength + len(p.SLI)),
	}
	hData, err := h.Marshal()
	if err != nil {
		return nil, err
	}

	return append(hData, rawPacket...), nil
}

// Unmarshal decodes the SliceLossIndication from binary
func (p *SliceLossIndication) Unmarshal(rawPacket []byte) error {
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
	for i := headerLength + sliOffset; i < (headerLength + int(h.Length*4)); i += 4 {
		sli := binary.BigEndian.Uint32(rawPacket[i:])
		p.SLI = append(p.SLI, struct {
			First   uint16
			Number  uint16
			Picture uint8
		}{
			uint16((sli >> 19) & 0x1FFF),
			uint16((sli >> 6) & 0x1FFF),
			uint8(sli & 0x3F)})
	}
	return nil
}

func (p *SliceLossIndication) len() int {
	return headerLength + sliOffset + (len(p.SLI) * 4)
}

// Header returns the Header associated with this packet.
func (p *SliceLossIndication) Header() Header {
	return Header{
		Count:  sliFMT,
		Type:   TypeTransportSpecificFeedback,
		Length: uint16((p.len() / 4) - 1),
	}
}

func (p *SliceLossIndication) String() string {
	return fmt.Sprintf("SliceLossIndication %x %x %+v", p.SenderSSRC, p.MediaSSRC, p.SLI)
}

// DestinationSSRC returns an array of SSRC values that this packet refers to.
func (p *SliceLossIndication) DestinationSSRC() []uint32 {
	return []uint32{p.MediaSSRC}
}
