package srtp

import (
	"fmt"

	"github.com/pions/webrtc/pkg/rtp"
)

type readResultSRTP struct {
	len    int
	header *rtp.Header
}

// ReadStreamSRTP handles decryption for a single RTP SSRC
type ReadStreamSRTP struct {
	session   *SessionSRTP
	ssrc      uint32
	readCh    chan []byte
	readRetCh chan readResultSRTP
}

// ReadRTP reads and decrypts full RTP packet and its header from the nextConn
func (r *ReadStreamSRTP) ReadRTP(payload []byte) (int, *rtp.Header, error) {
	select {
	case <-r.session.closed:
		return 0, nil, fmt.Errorf("SRTP session is closed")
	case r.readCh <- payload:
	}

	select {
	case <-r.session.closed:
		return 0, nil, fmt.Errorf("SRTP session is closed")
	case res := <-r.readRetCh:
		return res.len, res.header, nil
	}
}

// Read reads and decrypts full RTP packet from the nextConn
func (r *ReadStreamSRTP) Read(b []byte) (int, error) {
	select {
	case <-r.session.closed:
		return 0, fmt.Errorf("SRTP session is closed")
	case r.readCh <- b:
	}

	select {
	case <-r.session.closed:
		return 0, fmt.Errorf("SRTP session is closed")
	case res := <-r.readRetCh:
		return res.len, nil
	}
}

func (r *ReadStreamSRTP) init(child streamSession, ssrc uint32) error {
	sessionSRTP, ok := child.(*SessionSRTP)
	if !ok {
		return fmt.Errorf("ReadStreamSRTP init failed type assertion")
	}

	r.session = sessionSRTP
	r.ssrc = ssrc
	r.readCh = make(chan []byte)
	r.readRetCh = make(chan readResultSRTP)
	return nil
}

// GetSSRC returns the SSRC we are demuxing for
func (r *ReadStreamSRTP) GetSSRC() uint32 {
	return r.ssrc
}

// WriteStreamSRTP is stream for a single Session that is used to encrypt RTP
type WriteStreamSRTP struct {
	session *SessionSRTP
}

// WriteRTP encrypts a RTP header and its payload to the nextConn
func (w *WriteStreamSRTP) WriteRTP(header *rtp.Header, payload []byte) (int, error) {
	headerRaw, err := header.Marshal()
	if err != nil {
		return 0, err
	}

	return w.session.write(append(headerRaw, payload...))
}

// Write encrypts and writes a full RTP packets to the nextConn
func (w *WriteStreamSRTP) Write(b []byte) (int, error) {
	return w.session.write(b)
}
