package sctp

import (
	"github.com/pkg/errors"
)

/*
CookieAck represents an SCTP Chunk of type CookieAck

 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Type = 11   |Chunk  Flags   |     Length = 4                |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/
type CookieAck struct {
	ChunkHeader
}

// Unmarshal populates a CookieAck Chunk from a byte slice
func (c *CookieAck) Unmarshal(raw []byte) error {
	if err := c.ChunkHeader.Unmarshal(raw); err != nil {
		return err
	}

	if c.typ != COOKIEACK {
		return errors.Errorf("ChunkType is not of type COOKIEACK, actually is %s", c.typ.String())
	}

	return nil
}

// Marshal generates raw data from a CookieAck struct
func (c *CookieAck) Marshal() ([]byte, error) {
	c.ChunkHeader.typ = COOKIEACK
	return c.ChunkHeader.Marshal()
}

// Check asserts the validity of this structs values
func (c *CookieAck) Check() (abort bool, err error) {
	return false, nil
}
