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
	bufferSize int
	closedCh   chan struct{}
}

// NewMux creates a new Mux
func NewMux(conn net.Conn, bufferSize int) *Mux {
	m := &Mux{
		nextConn:   conn,
		endpoints:  make(map[*Endpoint]MatchFunc),
		bufferSize: bufferSize,
		closedCh:   make(chan struct{}),
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
	delete(m.endpoints, e)
}

// Close closes the Mux and all associated Endpoints.
func (m *Mux) Close() error {
	m.lock.Lock()
	for e := range m.endpoints {
		e.close()
		delete(m.endpoints, e)
	}
	m.lock.Unlock()

	err := m.nextConn.Close()
	if err != nil {
		return err
	}

	// Wait for readLoop to end
	<-m.closedCh

	return nil
}

func (m *Mux) readLoop() {
	defer func() {
		close(m.closedCh)
	}()
	buf := make([]byte, m.bufferSize)
	for {
		n, err := m.nextConn.Read(buf)
		if err != nil {
			return
		}

		m.dispatch(buf[:n])
	}
}

func (m *Mux) dispatch(buf []byte) {
	var endpoint *Endpoint

	m.lock.Lock()
	for e, f := range m.endpoints {
		if f(buf) {
			endpoint = e
			break
		}
	}
	m.lock.Unlock()

	if endpoint == nil {
		fmt.Printf("Warning: mux: no endpoint for packet starting with %d\n", buf[0])
		return
	}

	select {
	case readBuf, ok := <-endpoint.readCh:
		if !ok {
			return
		}
		n := copy(readBuf, buf)
		endpoint.wroteCh <- n
	case <-endpoint.doneCh:
		return
	}
}
