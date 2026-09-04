// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

// Package netconn provides the buffered net.Conn used at protocol boundaries.
package netconn

import (
	"net"
	"time"

	"github.com/pion/transport/v4/packetio"
)

const maxBufferSize = 1000 * 1000

// Config contains the outbound half of a Conn.
type Config struct {
	Write            func([]byte) (int, error)
	LocalAddr        func() net.Addr
	RemoteAddr       func() net.Addr
	SetWriteDeadline func(time.Time) error
	OnClose          func(*Conn)
}

// Conn adapts a pushed packet stream and outbound callbacks to net.Conn.
type Conn struct {
	config Config
	buffer *packetio.Buffer
}

// New creates a Conn with a bounded inbound buffer.
func New(config Config) *Conn {
	buffer := packetio.NewBuffer()
	buffer.SetLimitSize(maxBufferSize)

	return &Conn{config: config, buffer: buffer}
}

// Push supplies one inbound packet.
func (c *Conn) Push(packet []byte) error {
	_, err := c.buffer.Write(packet)

	return err
}

// Read reads one inbound packet.
func (c *Conn) Read(p []byte) (int, error) { return c.buffer.Read(p) }

// Write writes one outbound packet.
func (c *Conn) Write(p []byte) (int, error) { return c.config.Write(p) }

// Close closes the inbound side and runs OnClose once.
func (c *Conn) Close() error {
	_ = c.buffer.Close()
	if c.config.OnClose != nil {
		c.config.OnClose(c)
	}

	return nil
}

// LocalAddr returns the outbound transport's local address.
func (c *Conn) LocalAddr() net.Addr { return c.config.LocalAddr() }

// RemoteAddr returns the outbound transport's remote address.
func (c *Conn) RemoteAddr() net.Addr { return c.config.RemoteAddr() }

// SetDeadline sets both directions' deadlines.
func (c *Conn) SetDeadline(deadline time.Time) error {
	if err := c.SetReadDeadline(deadline); err != nil {
		return err
	}

	return c.SetWriteDeadline(deadline)
}

// SetReadDeadline sets the inbound buffer's deadline.
func (c *Conn) SetReadDeadline(deadline time.Time) error {
	return c.buffer.SetReadDeadline(deadline)
}

// SetWriteDeadline sets the outbound transport's deadline.
func (c *Conn) SetWriteDeadline(deadline time.Time) error {
	return c.config.SetWriteDeadline(deadline)
}
