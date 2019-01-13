package srtp

import (
	"fmt"

	"github.com/pions/webrtc/pkg/rtcp"
)

// ReadStreamSRTCP handles decryption for a single RTCP SSRC
type ReadStreamSRTCP struct {
	session *SessionSRTCP
	ssrc    uint32
}

// ReadRTCP reads and decrypts full RTCP packet and its header from the nextConn
func (r *ReadStreamSRTCP) ReadRTCP(payload []byte) (int, *rtcp.Header, error) {
	select {
	case <-r.session.closed:
		return 0, nil, fmt.Errorf("SRTCP session is closed")
	case r.session.readCh <- payload:
	}

	select {
	case <-r.session.closed:
		return 0, nil, fmt.Errorf("SRTCP session is closed")
	case res := <-r.session.readRetCh:
		return res.len, res.header, nil
	}
}

// Read reads and decrypts full RTCP packet from the nextConn
func (r *ReadStreamSRTCP) Read(b []byte) (int, error) {
	select {
	case <-r.session.closed:
		return 0, fmt.Errorf("SRTCP session is closed")
	case r.session.readCh <- b:
	}

	select {
	case <-r.session.closed:
		return 0, fmt.Errorf("SRTCP session is closed")
	case res := <-r.session.readRetCh:
		return res.len, nil
	}
}

func (r *ReadStreamSRTCP) init(child streamSession, ssrc uint32) error {
	sessionSRTCP, ok := child.(*SessionSRTCP)
	if !ok {
		return fmt.Errorf("ReadStreamSRTCP init failed type assertion")
	}

	r.session = sessionSRTCP
	r.ssrc = ssrc
	return nil

}

// GetSSRC returns the SSRC we are demuxing for
func (r *ReadStreamSRTCP) GetSSRC() uint32 {
	return r.ssrc
}

// WriteStreamSRTCP is stream for a single Session that is used to encrypt RTCP
type WriteStreamSRTCP struct {
	session *SessionSRTCP
}

// WriteRTCP encrypts a RTCP header and its payload to the nextConn
func (w *WriteStreamSRTCP) WriteRTCP(header *rtcp.Header, payload []byte) (int, error) {
	headerRaw, err := header.Marshal()
	if err != nil {
		return 0, err
	}

	return w.session.write(append(headerRaw, payload...))
}

// Write encrypts and writes a full RTP packets to the nextConn
func (w *WriteStreamSRTCP) Write(b []byte) (int, error) {
	return w.session.write(b)
}
