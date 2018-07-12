package sctp

import (
	"encoding/binary"
	"hash/crc32"

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
	Chunks          []Chunk
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

	offset := packetHeaderSize
	for {
		// Exact match, no more chunks
		if offset == len(raw) {
			break
		} else if offset+chunkHeaderSize > len(raw) {
			return errors.Errorf("Unable to parse SCTP chunk, not enough data for complete header: offset %d remaining %d", offset, len(raw))
		}

		var c Chunk
		switch ChunkType(raw[offset]) {
		case INIT:
			c = &Init{}
		case INITACK:
			c = &InitAck{}
		case ABORT:
			c = &Abort{}
		default:
			return errors.Errorf("Failed to unmarshal, contains unknown chunk type %s", ChunkType(raw[offset]).String())
		}

		if err := c.Unmarshal(raw[offset:]); err != nil {
			return err
		}

		p.Chunks = append(p.Chunks, c)
		chunkValuePadding := c.valueLength() % 4
		offset += chunkHeaderSize + c.valueLength() + chunkValuePadding
	}
	theirChecksum := binary.LittleEndian.Uint32(raw[8:])
	ourChecksum := generatePacketChecksum(raw)
	if theirChecksum != ourChecksum {
		return errors.Errorf("Checksum mismatch theirs: %d ours: %d", theirChecksum, ourChecksum)
	}
	return nil
}

// Marshal populates a raw buffer from a packet
func (p *Packet) Marshal() ([]byte, error) {
	raw := make([]byte, packetHeaderSize)

	// Populate static headers
	// 8-12 is Checksum which will be populated when packet is complete
	binary.BigEndian.PutUint16(raw[0:], p.SourcePort)
	binary.BigEndian.PutUint16(raw[2:], p.DestinationPort)
	binary.BigEndian.PutUint32(raw[4:], p.VerificationTag)

	// Populate chunks
	for _, c := range p.Chunks {
		chunkRaw, err := c.Marshal()
		if err != nil {
			return nil, err
		}
		raw = append(raw, chunkRaw...)
	}

	paddingNeeded := getPadding(len(raw), 4)
	if paddingNeeded != 0 {
		raw = append(raw, make([]byte, paddingNeeded)...)
	}

	// Checksum is already in BigEndian
	// Using LittleEndian.PutUint32 stops it from being flipped
	binary.LittleEndian.PutUint32(raw[8:], generatePacketChecksum(raw))
	return raw, nil
}

func generatePacketChecksum(raw []byte) uint32 {
	rawCopy := make([]byte, len(raw))
	copy(rawCopy, raw)

	// Clear existing checksum
	for offset := 8; offset <= 11; offset++ {
		rawCopy[offset] = 0x00
	}

	return crc32.Checksum(rawCopy, crc32.MakeTable(crc32.Castagnoli))
}
