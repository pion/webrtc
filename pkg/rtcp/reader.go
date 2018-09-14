package rtcp

import "io"

// A Reader reads packets from an RTCP combined packet.
type Reader struct {
	r io.Reader
}

// NewReader creates a new Reader reading from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{r}
}

// ReadPacket reads one packet from r.
//
// It returns the parsed packet Header and a byte slice containing the encoded
// packet data (including the header). How the packet data is parsed depends on
// the Type field contained in the Header.
func (r *Reader) ReadPacket() (header Header, data []byte, err error) {
	// First grab the header
	headerBuf := make([]byte, headerLength)
	if _, err := io.ReadFull(r.r, headerBuf); err != nil {
		return header, data, err
	}
	if err := header.Unmarshal(headerBuf); err != nil {
		return header, data, err
	}

	packetLen := (header.Length + 1) * 4

	// Then grab the rest
	bodyBuf := make([]byte, packetLen-headerLength)
	if _, err := io.ReadFull(r.r, bodyBuf); err != nil {
		return header, data, err
	}

	data = append(headerBuf, bodyBuf...)

	return header, data, nil
}
