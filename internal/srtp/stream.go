package srtp

import "fmt"

type streamSession interface {
	Close() error
	write([]byte) (int, error)
	decrypt([]byte) error
}

// ReadStream handles decryption for a single SSRC
type ReadStream struct {
	s streamSession

	decrypted chan []byte
	ssrc      uint32
}

// GetSSRC returns the SSRC this ReadStream gets data for
func (r *ReadStream) GetSSRC() uint32 {
	return r.ssrc
}

// Read reads decrypted packets from the stream
func (r *ReadStream) Read(buf []byte) (int, error) {
	decrypted, ok := <-r.decrypted
	if !ok {
		return 0, fmt.Errorf("Stream has been closed")
	} else if len(decrypted) > len(buf) {
		return 0, fmt.Errorf("Buffer is to small to copy")
	}

	copy(buf, decrypted)
	return len(decrypted), nil
}

// WriteStream is stream for a single Session that is used to encrypt
// RTP or RTCP
type WriteStream struct {
	session streamSession
}

// Write encrypts the passed RTP/RTCP buffer and writes to the session
func (w *WriteStream) Write(buf []byte) (int, error) {
	return w.session.write(buf)
}
