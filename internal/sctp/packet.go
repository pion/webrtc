package sctp

import (
	"encoding/binary"
	"fmt"
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
|     Source Value Number        |     Destination Value Number   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                      Verification Tag                         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                           Checksum                            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+


*/
type packet struct {
	sourcePort      uint16
	destinationPort uint16
	verificationTag uint32
	chunks          []chunk
}

const (
	packetHeaderSize = 12
)

func (p *packet) unmarshal(raw []byte) error {
	if len(raw) < packetHeaderSize {
		return errors.Errorf("raw only %d bytes, %d is the minimum length for a SCTP packet", len(raw), packetHeaderSize)
	}

	p.sourcePort = binary.BigEndian.Uint16(raw[0:])
	p.destinationPort = binary.BigEndian.Uint16(raw[2:])
	p.verificationTag = binary.BigEndian.Uint32(raw[4:])

	offset := packetHeaderSize
	for {
		// Exact match, no more chunks
		if offset == len(raw) {
			break
		} else if offset+chunkHeaderSize > len(raw) {
			return errors.Errorf("Unable to parse SCTP chunk, not enough data for complete header: offset %d remaining %d", offset, len(raw))
		}

		var c chunk
		switch chunkType(raw[offset]) {
		case INIT:
			c = &chunkInit{}
		case INITACK:
			c = &chunkInitAck{}
		case ABORT:
			c = &chunkAbort{}
		case COOKIEECHO:
			c = &chunkCookieEcho{}
		case COOKIEACK:
			c = &chunkCookieAck{}
		case HEARTBEAT:
			c = &chunkHeartbeat{}
		case PAYLOADDATA:
			c = &chunkPayloadData{}
		case SACK:
			c = &chunkSelectiveAck{}
		default:
			return errors.Errorf("Failed to unmarshal, contains unknown chunk type %s", chunkType(raw[offset]).String())
		}

		if err := c.unmarshal(raw[offset:]); err != nil {
			return err
		}

		p.chunks = append(p.chunks, c)
		chunkValuePadding := getPadding(c.valueLength())
		offset += chunkHeaderSize + c.valueLength() + chunkValuePadding
	}
	theirChecksum := binary.LittleEndian.Uint32(raw[8:])
	ourChecksum := generatePacketChecksum(raw)
	if theirChecksum != ourChecksum {
		return errors.Errorf("Checksum mismatch theirs: %d ours: %d", theirChecksum, ourChecksum)
	}
	return nil
}

func (p *packet) marshal() ([]byte, error) {
	raw := make([]byte, packetHeaderSize)

	// Populate static headers
	// 8-12 is Checksum which will be populated when packet is complete
	binary.BigEndian.PutUint16(raw[0:], p.sourcePort)
	binary.BigEndian.PutUint16(raw[2:], p.destinationPort)
	binary.BigEndian.PutUint32(raw[4:], p.verificationTag)

	// Populate chunks
	for _, c := range p.chunks {
		chunkRaw, err := c.marshal()
		if err != nil {
			return nil, err
		}
		raw = append(raw, chunkRaw...)

		paddingNeeded := getPadding(len(raw))
		if paddingNeeded != 0 {
			raw = append(raw, make([]byte, paddingNeeded)...)
		}
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

// String makes packet printable
func (p *packet) String() string {
	format := `Packet:
	sourcePort: %d
	destinationPort: %d
	verificationTag: %d
	`
	res := fmt.Sprintf(format,
		p.sourcePort,
		p.destinationPort,
		p.verificationTag,
	)
	for i, chunk := range p.chunks {
		res = res + fmt.Sprintf("Chunk %d:\n %s", i, chunk)
	}
	return res
}
