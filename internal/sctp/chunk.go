package sctp

import (
	"encoding/binary"
	"fmt"

	"github.com/pkg/errors"
)

type ChunkType uint8

const (
	DATA = 0
	INIT = 1
)

func (c ChunkType) String() string {
	switch c {
	case DATA:
		return "Payload data"
	case INIT:
		return "Initiation"
	default:
		return fmt.Sprintf("Unknown ChunkType: %d", c)
	}
}

/*
Chunk represents a SCTP Chunk, defined in https://tools.ietf.org/html/rfc4960#section-3.2
The figure below illustrates the field format for the chunks to be
transmitted in the SCTP packet.  Each chunk is formatted with a Chunk
Type field, a chunk-specific Flag field, a Chunk Length field, and a
Value field.

 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Chunk Type  | Chunk  Flags  |        Chunk Length           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|                          Chunk Value                          |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/
type Chunk struct {
	Type   ChunkType
	Flags  byte
	Length uint16
	Value  []byte
}

const (
	chunkHeaderSize = 4
)

func (c *Chunk) Unmarshal(raw []byte) error {
	c.Type = ChunkType(raw[0])
	c.Flags = byte(raw[1])
	c.Length = binary.BigEndian.Uint16(raw[2:])

	// Length includes Chunk header
	valueLength := int(c.Length - chunkHeaderSize)
	lengthAfterValue := len(raw) - (chunkHeaderSize + int(valueLength))

	if lengthAfterValue < 0 {
		return errors.Errorf("Not enough data left in SCTP packet to satisfy requested length remain %d req %d ", valueLength, len(raw)-chunkHeaderSize)
	} else if lengthAfterValue < 4 {
		// https://tools.ietf.org/html/rfc4960#section-3.2
		// The Chunk Length field does not count any chunk padding.
		// Chunks (including Type, Length, and Value fields) are padded out
		// by the sender with all zero bytes to be a multiple of 4 bytes
		// long.  This padding MUST NOT be more than 3 bytes in total.  The
		// Chunk Length value does not include terminating padding of the
		// chunk.  However, it does include padding of any variable-length
		// parameter except the last parameter in the chunk.  The receiver
		// MUST ignore the padding.
		for i := lengthAfterValue; i > 0; i-- {
			paddingOffset := chunkHeaderSize + valueLength + (i - 1)
			if raw[paddingOffset] != 0 {
				return errors.Errorf("Chunk padding is non-zero at offset %d ", paddingOffset)
			}
		}
	}

	c.Value = raw[chunkHeaderSize : chunkHeaderSize+valueLength]
	return nil
}
