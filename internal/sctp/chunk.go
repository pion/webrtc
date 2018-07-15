package sctp

import (
	"encoding/binary"
	"fmt"

	"github.com/pkg/errors"
)

// ChunkType is an enum for SCTP Chunk Type field
// This field identifies the type of information contained in the
// Chunk Value field.
type ChunkType uint8

// List of known ChunkType enums
const (
	PAYLOADDATA      ChunkType = 0
	INIT             ChunkType = 1
	INITACK          ChunkType = 2
	SACK             ChunkType = 3
	HEARTBEAT        ChunkType = 4
	HEARTBEATACK     ChunkType = 5
	ABORT            ChunkType = 6
	SHUTDOWN         ChunkType = 7
	SHUTDOWNACK      ChunkType = 8
	ERROR            ChunkType = 9
	COOKIEECHO       ChunkType = 10
	COOKIEACK        ChunkType = 11
	CWR              ChunkType = 13
	SHUTDOWNCOMPLETE ChunkType = 14
)

func (c ChunkType) String() string {
	switch c {
	case PAYLOADDATA:
		return "Payload data"
	case INIT:
		return "Initiation"
	case INITACK:
		return "Initiation Acknowledgement"
	case SACK:
		return "Selective Acknowledgement"
	case HEARTBEAT:
		return "Heartbeat"
	case HEARTBEATACK:
		return "Heartbeat Acknowledgement"
	case ABORT:
		return "Abort"
	case SHUTDOWN:
		return "Shutdown"
	case SHUTDOWNACK:
		return "Shutdown Acknowledgement"
	case ERROR:
		return "Error"
	case COOKIEECHO:
		return "Cookie Echo"
	case COOKIEACK:
		return "Cookie Acknowledgement"
	case CWR:
		return "Congestion Window Reduced"
	case SHUTDOWNCOMPLETE:
		return "Shutdown Complete"
	default:
		return fmt.Sprintf("Unknown ChunkType: %d", c)
	}
}

/*
ChunkHeader represents a SCTP Chunk header, defined in https://tools.ietf.org/html/rfc4960#section-3.2
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
type ChunkHeader struct {
	typ   ChunkType
	Flags byte
	raw   []byte
}

const (
	chunkHeaderSize = 4
)

// Unmarshal populates a ChunkHeader from a raw byte[]
func (c *ChunkHeader) Unmarshal(raw []byte) error {
	if len(raw) < chunkHeaderSize {
		return errors.Errorf("raw only %d bytes, %d is the minimum length for a SCTP chunk", len(raw), chunkHeaderSize)
	}

	c.typ = ChunkType(raw[0])
	c.Flags = byte(raw[1])
	length := binary.BigEndian.Uint16(raw[2:])

	// Length includes Chunk header
	valueLength := int(length - chunkHeaderSize)
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

	c.raw = raw[chunkHeaderSize : chunkHeaderSize+valueLength]
	return nil
}

// Marshal populates a raw byte[] from a ChunkHeader
func (c *ChunkHeader) Marshal() ([]byte, error) {
	raw := make([]byte, 4+len(c.raw))

	raw[0] = uint8(c.typ)
	raw[1] = c.Flags
	binary.BigEndian.PutUint16(raw[2:], uint16(len(c.raw)+chunkHeaderSize))
	copy(raw[4:], c.raw)
	return raw, nil
}

// Type returns the type of Chunk
func (c *ChunkHeader) Type() ChunkType {
	return c.typ
}

func (c *ChunkHeader) valueLength() int {
	return len(c.raw)
}

// Chunk represents an SCTP chunk
type Chunk interface {
	Unmarshal(raw []byte) error
	Marshal() ([]byte, error)
	Type() ChunkType
	Check() (bool, error)

	valueLength() int
}
