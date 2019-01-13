package srtp

import (
	"bytes"
	"fmt"
	"io"
	"net"

	"github.com/pions/webrtc/pkg/rtcp"
)

type readResultSRTCP struct {
	len    int
	header *rtcp.Header
}

// SessionSRTCP implements io.ReadWriteCloser and provides a bi-directional SRTCP session
// SRTCP itself does not have a design like this, but it is common in most applications
// for local/remote to each have their own keying material. This provides those patterns
// instead of making everyone re-implement
type SessionSRTCP struct {
	session
	writeStream *WriteStreamSRTCP
	readCh      chan []byte
	readRetCh   chan readResultSRTCP
}

// CreateSessionSRTCP creates a new SessionSRTCP
func CreateSessionSRTCP() *SessionSRTCP {
	s := &SessionSRTCP{
		readCh:    make(chan []byte),
		readRetCh: make(chan readResultSRTCP),
	}
	s.writeStream = &WriteStreamSRTCP{s}
	s.session.initalize()
	return s
}

// Start initializes any crypto context and allows reading/writing to begin
func (s *SessionSRTCP) Start(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt []byte, profile ProtectionProfile, nextConn net.Conn) error {
	s.session.nextConn = nextConn
	return s.session.start(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt, profile, s)
}

// OpenWriteStream returns the global write stream for the Session
func (s *SessionSRTCP) OpenWriteStream() (*WriteStreamSRTCP, error) {
	return s.writeStream, nil
}

// OpenReadStream opens a read stream for the given SSRC, it can be used
// if you want a certain SSRC, but don't want to wait for AcceptStream
func (s *SessionSRTCP) OpenReadStream(SSRC uint32) (*ReadStreamSRTCP, error) {
	r, _ := s.session.getOrCreateReadStream(SSRC, s, &ReadStreamSRTCP{})

	if readStream, ok := r.(*ReadStreamSRTCP); ok {
		return readStream, nil
	}
	return nil, fmt.Errorf("Failed to open ReadStreamSRCTP, type assertion failed")
}

// AcceptStream returns a stream to handle RTCP for a single SSRC
func (s *SessionSRTCP) AcceptStream() (*ReadStreamSRTCP, uint32, error) {
	stream, ok := <-s.newStream
	if !ok {
		return nil, 0, fmt.Errorf("SessionSRTCP has been closed")
	}

	readStream, ok := stream.(*ReadStreamSRTCP)
	if !ok {
		return nil, 0, fmt.Errorf("newStream was found, but failed type assertion")
	}

	return readStream, stream.GetSSRC(), nil
}

// Close ends the session
func (s *SessionSRTCP) Close() error {
	return s.session.close()
}

// Private

func (s *SessionSRTCP) write(buf []byte) (int, error) {
	if _, ok := <-s.session.started; ok {
		return 0, fmt.Errorf("started channel used incorrectly, should only be closed")
	}

	s.session.localContextMutex.Lock()
	defer s.session.localContextMutex.Unlock()

	encrypted, err := s.localContext.EncryptRTCP(nil, buf, nil)
	if err != nil {
		return 0, err
	}
	return s.session.nextConn.Write(encrypted)
}

func (s *SessionSRTCP) decrypt(buf []byte) error {
	decrypted, err := s.remoteContext.DecryptRTCP(nil, buf, nil)
	if err != nil {
		return err
	}

	compoundPacket := rtcp.NewReader(bytes.NewReader(decrypted))
	for {
		_, rawrtcp, err := compoundPacket.ReadPacket()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		var report rtcp.Packet
		report, _, err = rtcp.Unmarshal(rawrtcp)
		if err != nil {
			return err
		}

		for _, ssrc := range report.DestinationSSRC() {
			r, isNew := s.session.getOrCreateReadStream(ssrc, s, &ReadStreamSRTCP{})
			if r == nil {
				return nil // Session has been closed
			} else if isNew {
				s.session.newStream <- r // Notify AcceptStream
			}

			readBuf := <-s.readCh
			if len(readBuf) < len(decrypted) {
				return fmt.Errorf("Input buffer was not long enough to contain decrypted RTCP")
			}

			copy(readBuf, decrypted)
			h := report.Header()

			s.readRetCh <- readResultSRTCP{
				len:    len(decrypted),
				header: &h,
			}
		}
	}
}
