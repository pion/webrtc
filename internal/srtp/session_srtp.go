package srtp

import (
	"fmt"
	"net"

	"github.com/pions/webrtc/pkg/rtp"
)

// SessionSRTP implements io.ReadWriteCloser and provides a bi-directional SRTP session
// SRTP itself does not have a design like this, but it is common in most applications
// for local/remote to each have their own keying material. This provides those patterns
// instead of making everyone re-implement
type SessionSRTP struct {
	session
	writeStream *WriteStreamSRTP
}

// CreateSessionSRTP creates a new SessionSRTP
func CreateSessionSRTP() *SessionSRTP {
	s := &SessionSRTP{}
	s.writeStream = &WriteStreamSRTP{s}
	s.session.initalize()
	return s
}

// Start initializes any crypto context and allows reading/writing to begin
func (s *SessionSRTP) Start(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt []byte, profile ProtectionProfile, nextConn net.Conn) error {
	s.session.nextConn = nextConn
	return s.session.start(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt, profile, s)
}

// OpenWriteStream returns the global write stream for the Session
func (s *SessionSRTP) OpenWriteStream() (*WriteStreamSRTP, error) {
	return s.writeStream, nil
}

// OpenReadStream opens a read stream for the given SSRC, it can be used
// if you want a certain SSRC, but don't want to wait for AcceptStream
func (s *SessionSRTP) OpenReadStream(SSRC uint32) (*ReadStreamSRTP, error) {
	r, _ := s.session.getOrCreateReadStream(SSRC, s, &ReadStreamSRTP{})

	if readStream, ok := r.(*ReadStreamSRTP); ok {
		return readStream, nil
	}
	return nil, fmt.Errorf("Failed to open ReadStreamSRCTP, type assertion failed")
}

// AcceptStream returns a stream to handle RTCP for a single SSRC
func (s *SessionSRTP) AcceptStream() (*ReadStreamSRTP, uint32, error) {
	stream, ok := <-s.newStream
	if !ok {
		return nil, 0, fmt.Errorf("SessionSRTP has been closed")
	}

	readStream, ok := stream.(*ReadStreamSRTP)
	if !ok {
		return nil, 0, fmt.Errorf("newStream was found, but failed type assertion")
	}

	return readStream, stream.GetSSRC(), nil
}

// Close ends the session
func (s *SessionSRTP) Close() error {
	return s.session.close()
}

func (s *SessionSRTP) write(buf []byte) (int, error) {
	if _, ok := <-s.session.started; ok {
		return 0, fmt.Errorf("started channel used incorrectly, should only be closed")
	}

	s.session.localContextMutex.Lock()
	defer s.session.localContextMutex.Unlock()

	encrypted, err := s.localContext.EncryptRTP(nil, buf, nil)
	if err != nil {
		return 0, err
	}
	return s.session.nextConn.Write(encrypted)
}

func (s *SessionSRTP) decrypt(buf []byte) error {
	h := &rtp.Header{}
	if err := h.Unmarshal(buf); err != nil {
		return err
	}

	r, isNew := s.session.getOrCreateReadStream(h.SSRC, s, &ReadStreamSRTP{})
	if r == nil {
		return nil // Session has been closed
	} else if isNew {
		s.session.newStream <- r // Notify AcceptStream
	}

	readStream, ok := r.(*ReadStreamSRTP)
	if !ok {
		return fmt.Errorf("Failed to get/create ReadStreamSRTP")
	}

	readBuf := <-readStream.readCh
	decrypted, err := s.remoteContext.decryptRTP(readBuf, buf, h)
	if err != nil {
		return err
	} else if len(decrypted) > len(readBuf) {
		return fmt.Errorf("Input buffer was not long enough to contain decrypted RTP")
	}

	readStream.readRetCh <- readResultSRTP{
		len:    len(decrypted),
		header: h,
	}

	return nil
}
