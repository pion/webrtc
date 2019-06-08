package mux

import (
	"io"
	"net"
	"time"

	"github.com/pion/ice"
	"github.com/pion/transport/packetio"
)

// Endpoint implements net.Conn. It is used to read muxed packets.
type Endpoint struct {
	mux    *Mux
	buffer *packetio.Buffer
}

// Close unregisters the endpoint from the Mux
func (e *Endpoint) Close() (err error) {
	err = e.close()
	if err != nil {
		return err
	}

	e.mux.RemoveEndpoint(e)
	return nil
}

func (e *Endpoint) close() error {
	return e.buffer.Close()
}

// Read reads a packet of len(p) bytes from the underlying conn
// that are matched by the associated MuxFunc
func (e *Endpoint) Read(p []byte) (int, error) {
	return e.buffer.Read(p)
}

// Write writes len(p) bytes to the underlying conn
func (e *Endpoint) Write(p []byte) (int, error) {
	n, err := e.mux.nextConn.Write(p)
	if err == ice.ErrNoCandidatePairs {
		return 0, nil
	} else if err == ice.ErrClosed {
		return 0, io.ErrClosedPipe
	}

	return n, err
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
