package srtp

import (
	"fmt"
	"net"
	"sync"

	"github.com/pions/webrtc/pkg/rtp"
)

// SessionSRTP implements io.ReadWriteCloser and provides a bi-directional SRTP session
// SRTP itself does not have a design like this, but it is common in most applications
// for local/remote to each have their own keying material. This provides those patterns
// instead of making everyone re-implement
type SessionSRTP struct {
	session
	writeStream *WriteStream
}

// CreateSessionSRTP creates a new SessionSRTP
func CreateSessionSRTP(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt []byte, profile ProtectionProfile, nextConn net.Conn) (*SessionSRTP, error) {
	s := &SessionSRTP{
		session: session{nextConn: nextConn},
	}
	s.writeStream = &WriteStream{s}

	if err := s.session.initalize(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt, profile, s); err != nil {
		return nil, err
	}

	return s, nil
}

// OpenWriteStream returns the global write stream for the Session
func (s *SessionSRTP) OpenWriteStream() (*WriteStream, error) {
	return s.writeStream, nil
}

// OpenReadStream opens a read stream for the given SSRC, it can be used
// if you want a certain SSRC, but don't want to wait for AcceptStream
func (s *SessionSRTP) OpenReadStream(SSRC uint32) (*ReadStream, error) {
	r, _ := s.session.getOrCreateReadStream(SSRC, s)
	return r, nil
}

// AcceptStream returns a stream to handle RTCP for a single SSRC
func (s *SessionSRTP) AcceptStream() (*ReadStream, uint32, error) {
	stream, ok := <-s.newStream
	if !ok {
		return nil, 0, fmt.Errorf("SessionSRTP has been closed")
	}

	return stream, stream.GetSSRC(), nil
}

// Close ends the session
func (s *SessionSRTP) Close() error {
	return nil
}

func (s *SessionSRTP) write(buf []byte) (int, error) {
	s.session.localContextMutex.Lock()
	defer s.session.localContextMutex.Unlock()

	encrypted, err := s.localContext.EncryptRTP(buf)
	if err != nil {
		return 0, err
	}
	return s.session.nextConn.Write(encrypted)
}

func (s *SessionSRTP) decrypt(buf []byte) error {
	decrypted, err := s.remoteContext.DecryptRTP(buf)
	if err != nil {
		return err
	}

	p := &rtp.Packet{}
	if err := p.Unmarshal(decrypted); err != nil {
		return err
	}

	r, isNew := s.session.getOrCreateReadStream(p.SSRC, s)
	if r == nil {
		return nil // Session has been closed
	} else if isNew {
		s.session.newStream <- r // Notify AcceptStream
	}

	r.decrypted <- decrypted
	return nil
}

// SessionSRTCP implements io.ReadWriteCloser and provides a bi-directional SRTP session
// SRTP itself does not have a design like this, but it is common in most applications
// for local/remote to each have their own keying material. This provides those patterns
// instead of making everyone re-implement
type SessionSRTCP struct {
	session
	writeStream *WriteStream
}

// CreateSessionSRTCP creates a new SessionSRTCP
func CreateSessionSRTCP(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt []byte, profile ProtectionProfile, nextConn net.Conn) (*SessionSRTCP, error) {
	s := &SessionSRTCP{
		session: session{nextConn: nextConn},
	}
	s.writeStream = &WriteStream{s}

	if err := s.session.initalize(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt, profile, s); err != nil {
		return nil, err
	}

	return s, nil
}

// OpenWriteStream returns the global write stream for the Session
func (s *SessionSRTCP) OpenWriteStream() (*WriteStream, error) {
	return s.writeStream, nil
}

// OpenReadStream opens a read stream for the given SSRC, it can be used
// if you want a certain SSRC, but don't want to wait for AcceptStream
func (s *SessionSRTCP) OpenReadStream(SSRC uint32) (*ReadStream, error) {
	r, _ := s.session.getOrCreateReadStream(SSRC, s)
	return r, nil
}

// AcceptStream returns a stream to handle RTCP for a single SSRC
func (s *SessionSRTCP) AcceptStream() (*ReadStream, uint32, error) {
	stream, ok := <-s.newStream
	if !ok {
		return nil, 0, fmt.Errorf("SessionSRTP has been closed")
	}

	return stream, stream.GetSSRC(), nil
}

// Close ends the session
func (s *SessionSRTCP) Close() error {
	return nil
}

func (s *SessionSRTCP) write(buf []byte) (int, error) {
	s.session.localContextMutex.Lock()
	defer s.session.localContextMutex.Unlock()

	encrypted, err := s.localContext.EncryptRTCP(buf)
	if err != nil {
		return 0, err
	}
	return s.session.nextConn.Write(encrypted)
}

func (s *SessionSRTCP) decrypt(buf []byte) error {
	fmt.Println("TODO SessionSRTCP.decrypt")
	// func handleRTCP(getBufferTransports func(uint32) *TransportPair, buffer []byte) {
	// 	//decrypted packets can also be compound packets, so we have to nest our reader loop here.
	// 	compoundPacket := rtcp.NewReader(bytes.NewReader(buffer))
	// 	for {
	// 		_, rawrtcp, err := compoundPacket.ReadPacket()
	//
	// 		if err != nil {
	// 			if err == io.EOF {
	// 				break
	// 			}
	// 			fmt.Println(err)
	// 			return
	// 		}
	//
	// 		var report rtcp.Packet
	// 		report, _, err = rtcp.Unmarshal(rawrtcp)
	// 		if err != nil {
	// 			fmt.Println(err)
	// 			return
	// 		}
	//
	// 		f := func(ssrc uint32) {
	// 			bufferTransport := getBufferTransports(ssrc)
	// 			if bufferTransport != nil && bufferTransport.RTCP != nil {
	// 				select {
	// 				case bufferTransport.RTCP <- report:
	// 				default:
	// 				}
	// 			}
	// 		}
	//
	// 		for _, ssrc := range report.DestinationSSRC() {
	// 			f(ssrc)
	// 		}
	// 	}
	// }
	return nil
}

/*
	Private
*/
type session struct {
	localContextMutex           sync.Mutex
	localContext, remoteContext *Context

	newStream chan *ReadStream

	readStreamsClosed bool
	readStreams       map[uint32]*ReadStream
	readStreamsLock   sync.Mutex

	nextConn net.Conn
}

func (s *session) getOrCreateReadStream(ssrc uint32, child streamSession) (*ReadStream, bool) {
	s.readStreamsLock.Lock()
	defer s.readStreamsLock.Unlock()

	if s.readStreamsClosed {
		return nil, false
	}

	isNew := false
	r, ok := s.readStreams[ssrc]
	if !ok {
		r = &ReadStream{s: child, decrypted: make(chan []byte), ssrc: ssrc}
		s.readStreams[ssrc] = r

		isNew = true
	}

	return r, isNew
}

func (s *session) initalize(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt []byte, profile ProtectionProfile, child streamSession) error {
	var err error
	s.localContext, err = CreateContext(localMasterKey, localMasterSalt, profile)
	if err != nil {
		return err
	}

	s.remoteContext, err = CreateContext(remoteMasterKey, remoteMasterSalt, profile)
	if err != nil {
		return err
	}

	s.readStreams = map[uint32]*ReadStream{}
	s.newStream = make(chan *ReadStream)

	go func() {
		defer func() {
			close(s.newStream)

			s.readStreamsLock.Lock()
			s.readStreamsClosed = true
			for _, r := range s.readStreams {
				close(r.decrypted)
			}
			s.readStreamsLock.Unlock()
		}()

		b := make([]byte, 8192)
		for {
			var i int
			i, err = s.nextConn.Read(b)
			if err != nil {
				fmt.Println(err)
				return
			}

			if err = child.decrypt(b[:i]); err != nil {
				fmt.Println(err)
				return
			}
		}
	}()
	return nil
}
