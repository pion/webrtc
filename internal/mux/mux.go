package mux

import (
	"fmt"
	"net"
	"sync"
)

// Mux allows multiplexing
type Mux struct {
	lock       sync.RWMutex
	nextConn   net.Conn
	endpoints  map[*Endpoint]MatchFunc
	doneCh     chan struct{}
	bufferSize int
}

// NewMux creates a new Mux
func NewMux(conn net.Conn, bufferSize int) *Mux {
	m := &Mux{
		nextConn:   conn,
		endpoints:  make(map[*Endpoint]MatchFunc),
		doneCh:     make(chan struct{}),
		bufferSize: bufferSize,
	}

	go m.readLoop()

	return m
}

// NewEndpoint creates a new Endpoint
func (m *Mux) NewEndpoint(f MatchFunc) *Endpoint {
	e := &Endpoint{
		mux:     m,
		readCh:  make(chan []byte),
		wroteCh: make(chan int),
		doneCh:  make(chan struct{}),
	}

	m.lock.Lock()
	m.endpoints[e] = f
	m.lock.Unlock()

	return e
}

// RemoveEndpoint removes an endpoint from the Mux
func (m *Mux) RemoveEndpoint(e *Endpoint) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.removeEndpoint(e)
}

// removeEndpoint removes an endpoint from the Mux
// The caller should hold the lock
func (m *Mux) removeEndpoint(e *Endpoint) {
	e.close()
	delete(m.endpoints, e)
}

// Close closes the Mux and all associated Endpoints.
func (m *Mux) Close() {
	m.lock.Lock()
	defer m.lock.Unlock()

	for e := range m.endpoints {
		m.removeEndpoint(e)
	}

	select {
	case <-m.doneCh:
	default:
		close(m.doneCh)
	}
}

func (m *Mux) readLoop() {
	buf := make([]byte, m.bufferSize)
	for {
		select {
		case <-m.doneCh:
			return
		default:
		}
		n, err := m.nextConn.Read(buf)
		if err != nil {
			return
		}

		m.dispatch(buf[:n])
	}
}

func (m *Mux) dispatch(buf []byte) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for e, f := range m.endpoints {
		if f(buf) {
			readBuf := <-e.readCh
			n := copy(readBuf, buf)
			e.wroteCh <- n
			return
		}
	}

	fmt.Printf("Warning: mux: no endpoint for packet starting with %d\n", buf[0])
}
