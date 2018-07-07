package sctp

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

/*
Packet represents an SCTP packet, defined in https://tools.ietf.org/html/rfc4960#section-3
An SCTP packet is composed of a common header and chunks.  A chunk
contains either control information or user data.


                      SCTP Packet Format
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                        Common Header                          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                          Chunk #1                             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                           ...                                 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                          Chunk #n                             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+


                SCTP Common Header Format

 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|     Source Port Number        |     Destination Port Number   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                      Verification Tag                         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                           Checksum                            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+


*/
type Packet struct {
	SourcePort      uint16
	DestinationPort uint16
	VerificationTag uint32
	Checksum        uint32
	Chunks          []*Chunk
}

const (
	packetHeaderSize = 12
)

// Unmarshal populates a Packet from a raw buffer
func (p *Packet) Unmarshal(raw []byte) error {
	if len(raw) < packetHeaderSize {
		return errors.Errorf("raw only %d bytes, %d is the minimum length for a SCTP packet", len(raw), packetHeaderSize)
	}

	p.SourcePort = binary.BigEndian.Uint16(raw[0:])
	p.DestinationPort = binary.BigEndian.Uint16(raw[2:])
	p.VerificationTag = binary.BigEndian.Uint32(raw[4:])
	p.Checksum = binary.BigEndian.Uint32(raw[8:])

	offset := packetHeaderSize
	for {
		// Exact match, no more chunks
		if offset == len(raw) {
			break
		} else if offset+chunkHeaderSize > len(raw) {
			return errors.Errorf("Unable to parse SCTP chunk, not enough data for complete header: offset %d remaining %d", offset, len(raw))
		}

		c := &Chunk{}
		if err := c.Unmarshal(raw[offset:]); err != nil {
			return err
		}
		p.Chunks = append(p.Chunks, c)

		chunkValuePadding := len(c.Value) % 4
		offset += chunkHeaderSize + len(c.Value) + chunkValuePadding
	}
	return nil
}
