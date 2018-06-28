package rtp

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

// Packet represents an RTP Packet
// RTP is a network protocol for delivering audio and video over IP networks.
type Packet struct {
	Raw              []byte
	Version          uint8
	Padding          bool
	Extension        bool
	Marker           bool
	PayloadOffset    int
	PayloadType      uint8
	SequenceNumber   uint16
	Timestamp        uint32
	SSRC             uint32
	CSRC             []uint32
	ExtensionProfile uint16
	ExtensionPayload []byte
	Payload          []byte
}

const (
	headerLength    = 4
	versionShift    = 6
	versionMask     = 0x3
	paddingShift    = 5
	paddingMask     = 0x1
	extensionShift  = 4
	extensionMask   = 0x1
	ccMask          = 0xF
	markerShift     = 7
	markerMask      = 0x1
	ptMask          = 0x7F
	seqNumOffset    = 2
	seqNumLength    = 2
	timestampOffset = 4
	timestampLength = 4
	ssrcOffset      = 8
	ssrcLength      = 4
	csrcOffset      = 12
	csrcLength      = 4
)

// Unmarshal parses the passed byte slice and stores the result in the Packet this method is called upon
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

	currOffset := csrcOffset + (len(p.CSRC) * csrcLength)
	if len(rawPacket) < currOffset {
		return errors.Errorf("RTP header size insufficient; %d < %d", len(rawPacket), currOffset)
	}

	for i := range p.CSRC {
		offset := csrcOffset + (i * csrcLength)
		p.CSRC[i] = binary.BigEndian.Uint32(rawPacket[offset:offset])
	}

	if p.Extension {
		p.ExtensionProfile = binary.BigEndian.Uint16(rawPacket[currOffset:])
		currOffset += 2
		extensionLength := binary.BigEndian.Uint16(rawPacket[currOffset:])
		currOffset += 2
		p.ExtensionPayload = rawPacket[currOffset : currOffset+int(extensionLength)]
		currOffset += len(p.ExtensionPayload) * 4
	}

	p.Payload = rawPacket[currOffset:]
	p.PayloadOffset = currOffset
	p.Raw = rawPacket
	return nil
}

// Marshal returns a raw RTP packet for the instance it is called upon
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

	rawPacketLength := 12 + (len(p.CSRC) * csrcLength)
	if p.Extension {
		rawPacketLength += 4 + len(p.ExtensionPayload)
	}
	rawPacket := make([]byte, rawPacketLength)

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
		binary.BigEndian.PutUint16(rawPacket[currOffset:], uint16(len(p.ExtensionPayload))/4)
		currOffset += 2
		copy(rawPacket[currOffset:], p.ExtensionPayload)
	}

	p.PayloadOffset = csrcOffset + (len(p.CSRC) * csrcLength)

	rawPacket = append(rawPacket, p.Payload...)
	p.Raw = rawPacket

	return rawPacket, nil
}
