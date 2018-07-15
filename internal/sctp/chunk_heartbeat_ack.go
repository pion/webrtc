package sctp

import (
	"github.com/pkg/errors"
)

/*
HeartbeatAck represents an SCTP Chunk of type HEARTBEAT ACK

An endpoint should send this chunk to its peer endpoint as a response
to a HEARTBEAT chunk (see Section 8.3).  A HEARTBEAT ACK is always
sent to the source IP address of the IP datagram containing the
HEARTBEAT chunk to which this ack is responding.

The parameter field contains a variable-length opaque data structure.

 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Type = 5    | Chunk  Flags  |    Heartbeat Ack Length       |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|            Heartbeat Information TLV (Variable-Length)        |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+


Defined as a variable-length parameter using the format described
in Section 3.2.1, i.e.:

Variable Parameters                  Status     Type Value
-------------------------------------------------------------
Heartbeat Info                       Mandatory   1

*/
type HeartbeatAck struct {
	ChunkHeader
	params []Param
}

// Unmarshal populates a HeartbeatAck Chunk from a byte slice
func (h *HeartbeatAck) Unmarshal(raw []byte) error {
	return errors.Errorf("Unimplemented")
}

// Marshal generates raw data from a HeartbeatAck struct
func (h *HeartbeatAck) Marshal() ([]byte, error) {
	if len(h.params) != 1 {
		return nil, errors.Errorf("HeartbeatAck must have one param")
	}

	switch h.params[0].(type) {
	case *ParamHeartbeatInfo:
		// ParamHeartbeatInfo is valid
	default:
		return nil, errors.Errorf("HeartbeatAck must have one param, and it should be a HeartbeatInfo")

	}

	out := make([]byte, 0)
	for idx, p := range h.params {
		pp, err := p.Marshal()
		if err != nil {
			return nil, errors.Wrap(err, "Unable to marshal parameter for HeartbeatAck")
		}

		out = append(out, pp...)

		// Chunks (including Type, Length, and Value fields) are padded out
		// by the sender with all zero bytes to be a multiple of 4 bytes
		// long.  This padding MUST NOT be more than 3 bytes in total.  The
		// Chunk Length value does not include terminating padding of the
		// chunk.  *However, it does include padding of any variable-length
		// parameter except the last parameter in the chunk.*  The receiver
		// MUST ignore the padding.
		if idx != len(h.params)-1 {
			padding := make([]byte, getPadding(len(pp), 4))
			out = append(out, padding...)
		}
	}

	h.ChunkHeader.typ = HEARTBEATACK
	h.ChunkHeader.raw = out

	return h.ChunkHeader.Marshal()
}

// Check asserts the validity of this structs values
func (h *HeartbeatAck) Check() (abort bool, err error) {
	return false, nil
}
