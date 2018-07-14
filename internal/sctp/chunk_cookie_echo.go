package sctp

import (
	"github.com/pkg/errors"
)

/*
CookieEcho represents an SCTP Chunk of type CookieEcho

 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Type = 10   |Chunk  Flags   |         Length                |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                     Cookie                                    |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

*/
type CookieEcho struct {
	ChunkHeader
	Cookie []byte
}

// Unmarshal populates a CookieEcho Chunk from a byte slice
func (c *CookieEcho) Unmarshal(raw []byte) error {
	if err := c.ChunkHeader.Unmarshal(raw); err != nil {
		return err
	}

	if c.typ != COOKIEECHO {
		return errors.Errorf("ChunkType is not of type COOKIEECHO, actually is %s", c.typ.String())
	}
	c.Cookie = raw[chunkHeaderSize:]

	return nil
}

// Marshal generates raw data from a CookieEcho struct
func (c *CookieEcho) Marshal() ([]byte, error) {
	c.ChunkHeader.typ = COOKIEECHO
	c.ChunkHeader.raw = c.Cookie
	return c.ChunkHeader.Marshal()
}

// Check asserts the validity of this structs values
func (c *CookieEcho) Check() (abort bool, err error) {
	return false, nil
}
