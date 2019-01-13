package srtp

import (
	"fmt"
	"net"
	"sync"
)

type streamSession interface {
	Close() error
	write([]byte) (int, error)
	decrypt([]byte) error
}

type session struct {
	localContextMutex           sync.Mutex
	localContext, remoteContext *Context

	newStream chan readStream

	started chan interface{}
	closed  chan interface{}

	readStreamsClosed bool
	readStreams       map[uint32]readStream
	readStreamsLock   sync.Mutex

	nextConn net.Conn
}

func (s *session) getOrCreateReadStream(ssrc uint32, child streamSession, proto readStream) (readStream, bool) {
	s.readStreamsLock.Lock()
	defer s.readStreamsLock.Unlock()

	if s.readStreamsClosed {
		return nil, false
	}

	isNew := false
	r, ok := s.readStreams[ssrc]
	if !ok {
		if err := proto.init(child, ssrc); err != nil {
			return nil, false
		}

		r = proto
		isNew = true
		s.readStreams[ssrc] = r
	}

	return r, isNew
}

func (s *session) initalize() {
	s.readStreams = map[uint32]readStream{}
	s.newStream = make(chan readStream)
	s.started = make(chan interface{})
	s.closed = make(chan interface{})
}

func (s *session) close() error {
	if s.nextConn == nil {
		return nil
	} else if err := s.nextConn.Close(); err != nil {
		return err
	}

	<-s.closed
	return nil
}

func (s *session) start(localMasterKey, localMasterSalt, remoteMasterKey, remoteMasterSalt []byte, profile ProtectionProfile, child streamSession) error {
	var err error
	s.localContext, err = CreateContext(localMasterKey, localMasterSalt, profile)
	if err != nil {
		return err
	}

	s.remoteContext, err = CreateContext(remoteMasterKey, remoteMasterSalt, profile)
	if err != nil {
		return err
	}

	go func() {
		defer func() {
			close(s.newStream)

			s.readStreamsLock.Lock()
			s.readStreamsClosed = true
			s.readStreamsLock.Unlock()
			close(s.closed)
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

	close(s.started)

	return nil
}
