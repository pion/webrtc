package rtcp

import (
	"encoding/binary"
)

// The RapidResynchronizationRequest packet informs the encoder about the loss of an undefined amount of coded video data belonging to one or more pictures
type RapidResynchronizationRequest struct {
	// SSRC of sender
	SenderSSRC uint32

	// SSRC of the media source
	MediaSSRC uint32
}

const (
	rrrFMT    = 5
	rrrLength = 2
)

// Marshal encodes the RapidResynchronizationRequest in binary
func (p RapidResynchronizationRequest) Marshal() ([]byte, error) {
	/*
	 * RRR does not require parameters.  Therefore, the length field MUST be
	 * 2, and there MUST NOT be any Feedback Control Information.
	 *
	 * The semantics of this FB message is independent of the payload type.
	 */
	rawPacket := make([]byte, 8)
	binary.BigEndian.PutUint32(rawPacket, p.SenderSSRC)
	binary.BigEndian.PutUint32(rawPacket[4:], p.MediaSSRC)

	h := Header{
		Count:  rrrFMT,
		Type:   TypeTransportSpecificFeedback,
		Length: rrrLength,
	}
	hData, err := h.Marshal()
	if err != nil {
		return nil, err
	}

	return append(hData, rawPacket...), nil
}

// Unmarshal decodes the RapidResynchronizationRequest from binary
func (p *RapidResynchronizationRequest) Unmarshal(rawPacket []byte) error {

	if len(rawPacket) < (headerLength + (ssrcLength * 2)) {
		return errPacketTooShort
	}

	var h Header
	if err := h.Unmarshal(rawPacket); err != nil {
		return err
	}

	if h.Type != TypeTransportSpecificFeedback || h.Count != 1 {
		return errWrongType
	}

	p.SenderSSRC = binary.BigEndian.Uint32(rawPacket[headerLength:])
	p.MediaSSRC = binary.BigEndian.Uint32(rawPacket[headerLength+ssrcLength:])
	return nil
}
