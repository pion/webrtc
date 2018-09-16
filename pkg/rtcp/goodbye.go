package rtcp

import (
	"encoding/binary"
)

// The Goodbye packet indicates that one or more sources are no longer active.
type Goodbye struct {
	// The SSRC/CSRC identifiers that are no longer active
	Sources []uint32
	// Optional text indicating the reason for leaving, e.g., "camera malfunction" or "RTP loop detected"
	Reason string
}

// Marshal encodes the Goodbye packet in binary
func (g Goodbye) Marshal() ([]byte, error) {
	/*
	 *        0                   1                   2                   3
	 *        0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *       +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *       |V=2|P|    SC   |   PT=BYE=203  |             length            |
	 *       +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *       |                           SSRC/CSRC                           |
	 *       +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *       :                              ...                              :
	 *       +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * (opt) |     length    |               reason for leaving            ...
	 *       +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	rawPacket := make([]byte, len(g.Sources)*ssrcLength)

	if len(g.Sources) > countMax {
		return nil, errTooManySources
	}

	for i, s := range g.Sources {
		binary.BigEndian.PutUint32(rawPacket[i*ssrcLength:], s)
	}

	if g.Reason != "" {
		reason := []byte(g.Reason)

		if len(reason) > sdesMaxOctetCount {
			return nil, errReasonTooLong
		}

		rawPacket = append(rawPacket, uint8(len(reason)))
		rawPacket = append(rawPacket, reason...)

		// align to 32-bit boundary
		if len(rawPacket)%4 != 0 {
			padCount := 4 - len(rawPacket)%4
			rawPacket = append(rawPacket, make([]byte, padCount)...)
		}
	}

	h := Header{
		Padding: false,
		Count:   uint8(len(g.Sources)),
		Type:    TypeGoodbye,
		Length:  uint16(headerLength + len(rawPacket)),
	}
	hData, err := h.Marshal()
	if err != nil {
		return nil, err
	}

	rawPacket = append(hData, rawPacket...)

	return rawPacket, nil
}

// Unmarshal decodes the Goodbye packet from binary
func (g *Goodbye) Unmarshal(rawPacket []byte) error {
	/*
	 *        0                   1                   2                   3
	 *        0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *       +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *       |V=2|P|    SC   |   PT=BYE=203  |             length            |
	 *       +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *       |                           SSRC/CSRC                           |
	 *       +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *       :                              ...                              :
	 *       +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * (opt) |     length    |               reason for leaving            ...
	 *       +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	var header Header
	if err := header.Unmarshal(rawPacket); err != nil {
		return err
	}

	if header.Type != TypeGoodbye {
		return errWrongType
	}

	if len(rawPacket)%4 != 0 {
		return errPacketTooShort
	}

	g.Sources = make([]uint32, header.Count)

	reasonOffset := int(headerLength + header.Count*ssrcLength)
	if reasonOffset > len(rawPacket) {
		return errPacketTooShort
	}

	for i := 0; i < int(header.Count); i++ {
		offset := headerLength + i*ssrcLength
		if offset > len(rawPacket) {
			return errPacketTooShort
		}

		g.Sources[i] = binary.BigEndian.Uint32(rawPacket[offset:])
	}

	if reasonOffset < len(rawPacket) {
		reasonLen := int(rawPacket[reasonOffset])
		reasonEnd := reasonOffset + 1 + reasonLen

		if reasonEnd > len(rawPacket) {
			return errPacketTooShort
		}

		g.Reason = string(rawPacket[reasonOffset+1 : reasonEnd])

	}

	return nil
}
