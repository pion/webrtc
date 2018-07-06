package sctp

type ChunkType uint8

const (
	DATA = 0
)

func (c ChunkType) String() string {
	switch c {
	case DATA:
		return "Payload data"
	default:
		return "Unknown"
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
