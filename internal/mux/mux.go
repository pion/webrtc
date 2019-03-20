package mux

import (
	"net"
	"sync"

	"github.com/pions/logging"
	"github.com/pions/transport/packetio"
)

// The maximum amount of data that can be buffered before returning errors.
const maxBufferSize = 1000 * 1000 // 1MB

// Mux allows multiplexing
type Mux struct {
	lock       sync.RWMutex
	nextConn   net.Conn
	endpoints  map[*Endpoint]MatchFunc
	bufferSize int
	closedCh   chan struct{}

	log *logging.LeveledLogger
}

// NewMux creates a new Mux
func NewMux(conn net.Conn, bufferSize int) *Mux {
	m := &Mux{
		nextConn:   conn,
		endpoints:  make(map[*Endpoint]MatchFunc),
		bufferSize: bufferSize,
		closedCh:   make(chan struct{}),
		log:        logging.NewScopedLogger("mux"),
	}

	go m.readLoop()

	return m
}

// NewEndpoint creates a new Endpoint
func (m *Mux) NewEndpoint(f MatchFunc) *Endpoint {
	e := &Endpoint{
		mux:    m,
		buffer: packetio.NewBuffer(),
	}

	// Set a maximum size of the buffer in bytes.
	// NOTE: We actually won't get anywhere close to this limit.
	// SRTP will constantly read from the endpoint and drop packets if it's full.
	e.buffer.SetLimitSize(maxBufferSize)

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
		err := e.close()
		if err != nil {
			return err
		}

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

		err = m.dispatch(buf[:n])
		if err != nil {
			return
		}
	}
}

func (m *Mux) dispatch(buf []byte) error {
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
		m.log.Warnf("Warning: mux: no endpoint for packet starting with %d\n", buf[0])
		return nil
	}

	_, err := endpoint.buffer.Write(buf)
	if err != nil {
		return err
	}

	return nil
}
