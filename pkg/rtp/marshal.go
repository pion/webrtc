package rtp

import (
	"encoding/binary"
)

func (p *Packet) Marshal() ([]byte, error) {

	/*
	 *  0                   1                   2                   3
	 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |V=2|P|X|  CC   |M|     PT      |       sequence number         |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |                           timestamp                           |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |           synchronization source (SSRC) identifier            |
	 * +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * |            contributing source (CSRC) identifiers             |
	 * |                             ....                              |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	rawPacket := make([]byte, 12+(len(p.CSRC)*csrcLength)+(4+len(p.ExtensionPayload)))

	rawPacket[0] |= p.Version << versionShift
	if p.Padding {
		rawPacket[0] |= 1 << paddingShift
	}
	if p.Extension {
		rawPacket[0] |= 1 << extensionShift
	}
	rawPacket[0] |= uint8(len(p.CSRC))

	if p.Marker {
		rawPacket[1] |= 1 << markerShift
	}
	rawPacket[1] |= p.PayloadType

	binary.BigEndian.PutUint16(rawPacket[seqNumOffset:], p.SequenceNumber)
	binary.BigEndian.PutUint32(rawPacket[timestampOffset:], p.Timestamp)
	binary.BigEndian.PutUint32(rawPacket[ssrcOffset:], p.SSRC)

	for i, csrc := range p.CSRC {
		binary.BigEndian.PutUint32(rawPacket[csrcOffset+(i*csrcLength):], csrc)
	}

	currOffset := csrcOffset + (len(p.CSRC) * csrcLength)

	for i := range p.CSRC {
		offset := csrcOffset + (i * csrcLength)
		p.CSRC[i] = binary.BigEndian.Uint32(rawPacket[offset:offset])
	}

	if p.Extension {
		binary.BigEndian.PutUint16(rawPacket[currOffset:], p.ExtensionProfile)
		currOffset += 2
		binary.BigEndian.PutUint16(rawPacket[currOffset:], uint16(len(p.ExtensionPayload)) / 4)
		currOffset += 2
		copy(rawPacket[currOffset:], p.ExtensionPayload)
	}

	rawPacket = append(rawPacket, p.Payload...)

	return rawPacket, nil
}
