package mux

import (
	"errors"
	"net"
	"time"
)

// Endpoint implements net.Conn. It is used to read muxed packets.
type Endpoint struct {
	mux     *Mux
	readCh  chan []byte
	wroteCh chan int
	doneCh  chan struct{}
}

// Close unregisters the endpoint from the Mux
func (e *Endpoint) Close() error {
	e.close()
	e.mux.RemoveEndpoint(e)
	return nil
}

func (e *Endpoint) close() {
	select {
	case <-e.doneCh:
	default:
		close(e.doneCh)
	}
}

// Read reads a packet of len(p) bytes from the underlying conn
// that are matched by the associated MuxFunc
func (e *Endpoint) Read(p []byte) (int, error) {
	select {
	case e.readCh <- p:
		n := <-e.wroteCh
		return n, nil
	case <-e.doneCh:
		// Unblock Mux.dispatch
		select {
		case <-e.readCh:
		default:
			close(e.readCh)
		}
		return 0, errors.New("endpoint closed")
	}
}

// Write writes len(p) bytes to the underlying conn
func (e *Endpoint) Write(p []byte) (n int, err error) {
	return e.mux.nextConn.Write(p)
}

// LocalAddr is a stub
func (e *Endpoint) LocalAddr() net.Addr {
	return e.mux.nextConn.LocalAddr()
}

// RemoteAddr is a stub
func (e *Endpoint) RemoteAddr() net.Addr {
	return e.mux.nextConn.LocalAddr()
}

// SetDeadline is a stub
func (e *Endpoint) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline is a stub
func (e *Endpoint) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline is a stub
func (e *Endpoint) SetWriteDeadline(t time.Time) error {
	return nil
}
