package sctp

import (
	"github.com/pkg/errors"
)

/*
Abort represents an SCTP Chunk of type ABORT

The ABORT chunk is sent to the peer of an association to close the
association.  The ABORT chunk may contain Cause Parameters to inform
the receiver about the reason of the abort.  DATA chunks MUST NOT be
bundled with ABORT.  Control chunks (except for INIT, INIT ACK, and
SHUTDOWN COMPLETE) MAY be bundled with an ABORT, but they MUST be
placed before the ABORT in the SCTP packet or they will be ignored by
the receiver.

 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Type = 6    |Reserved     |T|           Length              |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|                   zero or more Error Causes                   |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/
type Abort struct {
	ChunkHeader
	ErrorCauses []ErrorCause
}

// Unmarshal populates a Abort Chunk from a byte slice
func (a *Abort) Unmarshal(raw []byte) error {
	if err := a.ChunkHeader.Unmarshal(raw); err != nil {
		return err
	}

	if a.typ != ABORT {
		return errors.Errorf("ChunkType is not of type ABORT, actually is %s", a.typ.String())
	}

	offset := chunkHeaderSize
	for {
		if len(raw)-offset < 4 {
			break
		}

		e, err := BuildErrorCause(raw[offset:])
		if err != nil {
			return errors.Wrap(err, "Failed build Abort Chunk")
		}

		offset += int(e.Length())
		a.ErrorCauses = append(a.ErrorCauses, e)
	}
	return nil
}

// Marshal generates raw data from a Abort struct
func (a *Abort) Marshal() ([]byte, error) {
	return nil, errors.Errorf("Unimplemented")
}

// Check asserts the validity of this structs values
func (a *Abort) Check() (abort bool, err error) {
	return false, nil
}
