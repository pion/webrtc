package srtp

import (
	"fmt"
	"net"
	"sync"
)

// SessionSRTP implements io.ReadWriteCloser and provides a bi-directional SRTP session
// SRTP itself does not have a design like this, but it is common in most applications
// for local/remote to each have their own keying material. This provides those patterns
// instead of making everyone re-implement
type SessionSRTP struct {
	session
}

// CreateSessionSRTP creates a new SessionSRTP
func CreateSessionSRTP(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt []byte, profile ProtectionProfile, nextConn net.Conn) (*SessionSRTP, error) {
	s := &SessionSRTP{
		session{nextConn: nextConn, toRead: make(chan []byte)},
	}

	if err := s.session.initalize(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt, profile /* isRTP */, true); err != nil {
		return nil, err
	}

	return s, nil
}

// Read reads from the session and decrypts to RTP
func (s *SessionSRTP) Read(buf []byte) (int, error) {
	decrypted, ok := <-s.toRead
	if !ok {
		return 0, fmt.Errorf("SessionSRTP has been closed")
	} else if len(decrypted) > len(buf) {
		return 0, fmt.Errorf("Buffer is to small to return RTP")
	}

	copy(buf, decrypted)
	return len(decrypted), nil
}

// Write encrypts the passed RTP buffer and writes to the session
func (s *SessionSRTP) Write(buf []byte) (int, error) {
	s.session.localContextMutex.Lock()
	defer s.session.localContextMutex.Unlock()

	encrypted, err := s.localContext.EncryptRTP(buf)
	if err != nil {
		return 0, err
	}
	return s.session.nextConn.Write(encrypted)
}

// Close ends the session
func (s *SessionSRTP) Close() error {
	return nil
}

// SessionSRTCP implements io.ReadWriteCloser and provides a bi-directional SRTP session
// SRTP itself does not have a design like this, but it is common in most applications
// for local/remote to each have their own keying material. This provides those patterns
// instead of making everyone re-implement
type SessionSRTCP struct {
	session
}

// CreateSessionSRTCP creates a new SessionSRTCP
func CreateSessionSRTCP(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt []byte, profile ProtectionProfile, nextConn net.Conn) (*SessionSRTCP, error) {
	s := &SessionSRTCP{
		session{nextConn: nextConn, toRead: make(chan []byte)},
	}

	if err := s.session.initalize(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt, profile /* isRTP */, false); err != nil {
		return nil, err
	}

	return s, nil
}

// Read reads from the session and decrypts to RTCP
func (s *SessionSRTCP) Read(buf []byte) (int, error) {
	decrypted, ok := <-s.toRead
	if !ok {
		return 0, fmt.Errorf("SessionSRTCP has been closed")
	} else if len(decrypted) > len(buf) {
		return 0, fmt.Errorf("Buffer is to small to return RTCP")
	}

	copy(buf, decrypted)
	return len(decrypted), nil
}

// Write encrypts the passed RTCP buffer and writes to the session
func (s *SessionSRTCP) Write(buf []byte) (int, error) {
	s.session.localContextMutex.Lock()
	defer s.session.localContextMutex.Unlock()

	encrypted, err := s.localContext.EncryptRTCP(buf)
	if err != nil {
		return 0, err
	}
	return s.session.nextConn.Write(encrypted)
}

// Close ends the session
func (s *SessionSRTCP) Close() error {
	return nil
}

/*
	Private
*/
type session struct {
	localContextMutex           sync.Mutex
	localContext, remoteContext *Context

	toRead chan []byte

	nextConn net.Conn
}

func (s *session) initalize(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt []byte, profile ProtectionProfile, isRTP bool) error {
	var err error
	s.localContext, err = CreateContext(localMasterKey, localMasterSalt, profile)
	if err != nil {
		return err
	}
	s.remoteContext, err = CreateContext(remoteMasterKey, remoteMasterSalt, profile)

	var decryptFunc func([]byte) ([]byte, error)
	if isRTP {
		decryptFunc = s.remoteContext.DecryptRTP
	} else {
		decryptFunc = s.remoteContext.DecryptRTCP
	}

	if err == nil {
		go func() {
			defer func() {
				close(s.toRead)
			}()

			b := make([]byte, 8192)
			for {
				var i int
				i, err = s.nextConn.Read(b)
				if err != nil {
					fmt.Println(err)
					return
				}

				var decrypted []byte
				decrypted, err = decryptFunc(b[:i])
				if err != nil {
					fmt.Println(err)
					return
				}

				s.toRead <- decrypted
			}
		}()
	}
	return err
}
