package sctp

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

/*
Heartbeat represents an SCTP Chunk of type HEARTBEAT

An endpoint should send this chunk to its peer endpoint to probe the
reachability of a particular destination transport address defined in
the present association.

The parameter field contains the Heartbeat Information, which is a
variable-length opaque data structure understood only by the sender.


 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Type = 4    | Chunk  Flags  |      Heartbeat Length         |
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
type Heartbeat struct {
	ChunkHeader
	params []Param
}

// Unmarshal populates a Abort Chunk from a byte slice
func (h *Heartbeat) Unmarshal(raw []byte) error {
	if err := h.ChunkHeader.Unmarshal(raw); err != nil {
		return err
	} else if h.typ != HEARTBEAT {
		return errors.Errorf("ChunkType is not of type HEARTBEAT, actually is %s", h.typ.String())
	}

	if len(raw) <= chunkHeaderSize {
		return errors.Errorf("Heartbeat is not long enough to contain Heartbeat Info %d", len(raw))
	}

	paramType := ParamType(binary.BigEndian.Uint16(raw[chunkHeaderSize:]))
	if paramType != HeartbeatInfo {
		return errors.Errorf("Heartbeat should only have HEARTBEAT param, instead have %s", paramType.String())
	}

	p, err := BuildParam(paramType, raw[chunkHeaderSize:])
	if err != nil {
		return errors.Wrap(err, "Failed unmarshalling param in Heartbeat Chunk")
	}
	h.params = append(h.params, p)

	return nil
}

// Marshal generates raw data from a Abort struct
func (h *Heartbeat) Marshal() ([]byte, error) {
	return nil, errors.Errorf("Unimplemented")
}

// Check asserts the validity of this structs values
func (h *Heartbeat) Check() (abort bool, err error) {
	return false, nil
}
