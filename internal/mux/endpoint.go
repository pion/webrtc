// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package mux

import (
	"errors"
	"io"
	"net"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/transport/v3/packetio"
)

// Endpoint implements net.Conn. It is used to read muxed packets.
type Endpoint struct {
	mux     *Mux
	buffer  *packetio.Buffer
	onClose func()
}

// Close unregisters the endpoint from the Mux.
func (e *Endpoint) Close() (err error) {
	if e.onClose != nil {
		e.onClose()
	}

	if err = e.close(); err != nil {
		return err
	}

	e.mux.RemoveEndpoint(e)

	return nil
}

func (e *Endpoint) close() error {
	return e.buffer.Close()
}

// Read reads a packet of len(p) bytes from the underlying conn
// that are matched by the associated MuxFunc.
func (e *Endpoint) Read(p []byte) (int, error) {
	return e.buffer.Read(p)
}

// ReadFrom reads a packet of len(p) bytes from the underlying conn
// that are matched by the associated MuxFunc.
func (e *Endpoint) ReadFrom(p []byte) (int, net.Addr, error) {
	i, err := e.Read(p)

	return i, nil, err
}

// Write writes len(p) bytes to the underlying conn.
func (e *Endpoint) Write(p []byte) (int, error) {
	n, err := e.mux.nextConn.Write(p)
	if errors.Is(err, ice.ErrNoCandidatePairs) {
		return 0, nil
	} else if errors.Is(err, ice.ErrClosed) {
		return 0, io.ErrClosedPipe
	}

	return n, err
}

// WriteTo writes len(p) bytes to the underlying conn.
func (e *Endpoint) WriteTo(p []byte, _ net.Addr) (int, error) {
	return e.Write(p)
}

// LocalAddr is a stub.
func (e *Endpoint) LocalAddr() net.Addr {
	return e.mux.nextConn.LocalAddr()
}

// RemoteAddr is a stub.
func (e *Endpoint) RemoteAddr() net.Addr {
	return e.mux.nextConn.RemoteAddr()
}

// SetDeadline sets the read deadline for this Endpoint.
// Write deadlines are not supported because writes go directly to the shared
// underlying connection and are non-blocking for this endpoint.
func (e *Endpoint) SetDeadline(t time.Time) error {
	return e.buffer.SetReadDeadline(t)
}

// SetReadDeadline sets the read deadline for this Endpoint.
func (e *Endpoint) SetReadDeadline(t time.Time) error {
	return e.buffer.SetReadDeadline(t)
}

// SetWriteDeadline is a stub.
func (e *Endpoint) SetWriteDeadline(time.Time) error {
	return nil
}

// SetOnClose is a user set callback that
// will be executed when `Close` is called.
func (e *Endpoint) SetOnClose(onClose func()) {
	e.onClose = onClose
}
