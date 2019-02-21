package webrtc

import (
	"fmt"
	"io"
	"sync"
)

// lossyReader wraps an io.Reader and discards data if it isn't read in time
// Allowing us to only deliver the newest data to the caller
type lossyReadCloser struct {
	nextReader io.ReadCloser
	mu         sync.RWMutex

	incomingBuf chan []byte
	amountRead  chan int

	readError  error
	hasErrored chan interface{}

	closed chan interface{}
}

func newLossyReadCloser(nextReader io.ReadCloser) *lossyReadCloser {
	l := &lossyReadCloser{
		nextReader: nextReader,

		closed: make(chan interface{}),

		incomingBuf: make(chan []byte),
		hasErrored:  make(chan interface{}),
		amountRead:  make(chan int),
	}

	go func() {
		readBuf := make([]byte, receiveMTU)
		for {
			i, err := nextReader.Read(readBuf)
			if err != nil {
				l.mu.Lock()
				l.readError = err
				l.mu.Unlock()

				close(l.hasErrored)
				break
			}

			select {
			case in := <-l.incomingBuf:
				copy(in, readBuf[:i])
				l.amountRead <- i
			default: // Discard if we have no inbound read
			}
		}
	}()

	return l
}

func (l *lossyReadCloser) Read(b []byte) (n int, err error) {
	select {
	case <-l.closed:
		return 0, fmt.Errorf("lossyReadCloser is closed")
	case <-l.hasErrored:
		l.mu.RLock()
		defer l.mu.RUnlock()
		return 0, l.readError

	case l.incomingBuf <- b:
	}

	select {
	case <-l.closed:
		return 0, fmt.Errorf("lossyReadCloser is closed")
	case <-l.hasErrored:
		l.mu.RLock()
		defer l.mu.RUnlock()
		return 0, l.readError

	case i := <-l.amountRead:
		return i, nil
	}
}

func (l *lossyReadCloser) Close() error {
	select {
	case <-l.closed:
		return fmt.Errorf("lossyReader is already closed")
	default:
	}
	close(l.closed)
	return l.nextReader.Close()
}
