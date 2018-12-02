package rtcp

import (
	"encoding/binary"
)

// The PictureLossIndication packet informs the encoder about the loss of an undefined amount of coded video data belonging to one or more pictures
type PictureLossIndication struct {
	// SSRC of sender
	SenderSSRC uint32

	// SSRC where the loss was experienced
	MediaSSRC uint32

	header Header
}

const (
	pliFMT    = 1
	pliLength = 2
)

// Marshal encodes the PictureLossIndication in binary
func (p PictureLossIndication) Marshal() ([]byte, error) {
	/*
	 * PLI does not require parameters.  Therefore, the length field MUST be
	 * 2, and there MUST NOT be any Feedback Control Information.
	 *
	 * The semantics of this FB message is independent of the payload type.
	 */
	rawPacket := make([]byte, p.len())
	packetBody := rawPacket[headerLength:]

	binary.BigEndian.PutUint32(packetBody, p.SenderSSRC)
	binary.BigEndian.PutUint32(packetBody[4:], p.MediaSSRC)

	h := Header{
		Count:  pliFMT,
		Type:   TypePayloadSpecificFeedback,
		Length: pliLength,
	}
	hData, err := h.Marshal()
	if err != nil {
		return nil, err
	}
	copy(rawPacket, hData)

	return rawPacket, nil
}

// Unmarshal decodes the PictureLossIndication from binary
func (p *PictureLossIndication) Unmarshal(rawPacket []byte) error {
	if len(rawPacket) < (headerLength + (ssrcLength * 2)) {
		return errPacketTooShort
	}

	var h Header
	if err := h.Unmarshal(rawPacket); err != nil {
		return err
	}

	if h.Type != TypePayloadSpecificFeedback || h.Count != pliFMT {
		return errWrongType
	}

	p.SenderSSRC = binary.BigEndian.Uint32(rawPacket[headerLength:])
	p.MediaSSRC = binary.BigEndian.Uint32(rawPacket[headerLength+ssrcLength:])
	return nil
}

// Header returns the Header associated with this packet.
func (p *PictureLossIndication) Header() Header {
	return p.header
}

func (p *PictureLossIndication) len() int {
	return headerLength + ssrcLength*2
}
