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
	s.localContextMutex.Lock()
	defer s.localContextMutex.Unlock()

	encrypted, err := s.localContext.EncryptRTP(buf)
	if err != nil {
		return 0, err
	}
	return s.session.nextConn.Write(encrypted)
}

// Close ends the session
func (s *SessionSRTP) Close() error {
	fmt.Println("SessionSRTP.Close() TODO")
	return nil
}

// CreateSessionSRTP creates a new SessionSRTP
func CreateSessionSRTP(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt []byte, profile ProtectionProfile, nextConn net.Conn) (*SessionSRTP, error) {
	s := &SessionSRTP{
		session{nextConn: nextConn, toRead: make(chan []byte)},
	}

	if err := s.session.initalize(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt, profile); err != nil {
		return nil, err
	}

	return s, nil
}

/*
	Private
*/
type session struct {
	localContextMutex           sync.Mutex
	localContext, remoteContext *Context

	toRead   chan []byte
	doneChan chan interface{}

	nextConn net.Conn
}

func (s *session) initalize(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt []byte, profile ProtectionProfile) error {
	var err error
	s.localContext, err = CreateContext(localMasterKey, localMasterSalt, profile)
	if err != nil {
		return err
	}
	s.remoteContext, err = CreateContext(remoteMasterKey, remoteMasterSalt, profile)

	if err == nil {
		go func() {
			defer func() {
				close(s.toRead)
			}()

			b := make([]byte, 8192)
			for {
				i, err := s.nextConn.Read(b)
				if err != nil {
					fmt.Println(err)
					return
				}

				decrypted, err := s.remoteContext.DecryptRTP(b[:i])
				if err != nil {
					fmt.Println(err)
					continue
					// return TODO until RTCP
				}

				s.toRead <- decrypted
			}
		}()
	}
	return err
}
