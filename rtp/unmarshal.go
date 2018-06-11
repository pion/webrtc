package rtp

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

func (p *Packet) Unmarshal(rawPacket []byte) error {
	if len(rawPacket) < headerLength {
		return errors.Errorf("RTP header size insufficient; %d < %d", len(rawPacket), headerLength)
	}

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

	p.Version = rawPacket[0] >> versionShift & versionMask
	p.Padding = (rawPacket[0] >> paddingShift & paddingMask) > 0
	p.Extension = (rawPacket[0] >> extensionShift & extensionMask) > 0
	p.CSRC = make([]uint32, rawPacket[0]&ccMask)

	p.Marker = (rawPacket[1] >> markerShift & markerMask) > 0
	p.PayloadType = rawPacket[1] & ptMask

	p.SequenceNumber = binary.BigEndian.Uint16(rawPacket[seqNumOffset : seqNumOffset+seqNumLength])
	p.Timestamp = binary.BigEndian.Uint32(rawPacket[timestampOffset : timestampOffset+timestampLength])
	p.SSRC = binary.BigEndian.Uint32(rawPacket[ssrcOffset : ssrcOffset+ssrcLength])

	currOffset := headerLength + timestampLength + ssrcLength + (len(p.CSRC) * csrcLength)
	if len(rawPacket) < currOffset {
		return errors.Errorf("RTP header size insufficient; %d < %d", len(rawPacket), currOffset)
	}

	for i := range p.CSRC {
		offset := csrcOffset + (i * csrcLength)
		p.CSRC[i] = binary.BigEndian.Uint32(rawPacket[offset:offset])
	}

	if p.Extension {
		currOffset += extensionHeaderIdLength
		extensionLength := binary.BigEndian.Uint16(rawPacket[currOffset:])
		currOffset += extensionLengthFieldLength

		currOffset += (int(extensionLength) * extensionHeaderAssumedLength)
	}

	p.Payload = rawPacket[currOffset:]

	return nil
}
