// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

// Package detacheddtls adapts DTLS application-data events to net.Conn while
// leaving DTLS datagram ownership with the caller.
package detacheddtls

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pion/dtls/v3"
	"github.com/pion/transport/v4/deadline"
	"github.com/pion/webrtc/v4/internal/netconn"
	"github.com/pion/webrtc/v4/internal/util"
)

// Config contains the transport operations used by a Conn.
type Config struct {
	WriteDatagram      func([]byte) (int, error)
	SetDatagramHandler func(func([]byte) error) func()
	NetConn            netconn.Config
	OnClose            func()
}

// Conn pumps a DetachedConn and exposes only its plaintext application data as
// net.Conn for SCTP.
type Conn struct {
	*netconn.Conn

	config Config

	// eventMu serializes event delivery with association replacement. Operations
	// needing both locks take eventMu before driveMu.
	driveMu sync.Mutex
	eventMu sync.Mutex

	dtlsConn      *dtls.DetachedConn
	handshakeDone chan error
	changed       chan struct{}
	ready         bool
	closed        bool
	closeErr      error
	writeDeadline *deadline.Deadline
}

// New creates a detached DTLS application-data connection.
func New(config Config) *Conn {
	c := &Conn{
		config: config, changed: make(chan struct{}), writeDeadline: deadline.New(),
	}
	config.NetConn.Write = c.write
	config.NetConn.SetWriteDeadline = func(t time.Time) error {
		c.writeDeadline.Set(t)

		return c.config.NetConn.SetWriteDeadline(t)
	}
	c.Conn = netconn.New(config.NetConn)
	go c.processEvents()

	return c
}

// Pause retires DTLS while ICE selects the replacement path.
func (c *Conn) Pause() error {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()
	c.driveMu.Lock()
	defer c.driveMu.Unlock()
	if c.closed {
		return io.ErrClosedPipe
	}
	_ = c.stop(false)
	c.config.SetDatagramHandler(nil)

	return nil
}

// Start replaces the DTLS association and waits for its handshake.
func (c *Conn) Start(ctx context.Context, conn *dtls.DetachedConn) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.eventMu.Lock()
	c.driveMu.Lock()
	if c.closed {
		c.driveMu.Unlock()
		c.eventMu.Unlock()

		return io.ErrClosedPipe
	}
	_ = c.stop(false)
	done := make(chan error, 1)
	c.dtlsConn, c.handshakeDone = conn, done
	err := conn.Start(ctx)
	var drain func()
	if err == nil {
		drain = c.config.SetDatagramHandler(func(packet []byte) error {
			return c.handleDatagram(conn, packet)
		})
	}
	c.driveMu.Unlock()
	c.eventMu.Unlock()
	if err != nil {
		return err
	}
	drain()

	return <-done
}

func (c *Conn) handleDatagram(conn *dtls.DetachedConn, datagram []byte) error {
	c.driveMu.Lock()
	defer c.driveMu.Unlock()
	if c.dtlsConn != conn {
		return nil
	}

	return conn.HandleDatagram(datagram, c.RemoteAddr())
}

func (c *Conn) processEvents() {
	for {
		c.eventMu.Lock()
		if c.closed {
			c.eventMu.Unlock()

			return
		}
		conn := c.dtlsConn
		var events <-chan struct{}
		if conn != nil {
			events = conn.EventReady()
		}
		if conn != nil {
			if err := c.processReadyEvents(conn); err != nil {
				c.driveMu.Lock()
				// Report handshake failure before OnClose initiates shutdown.
				c.notify(err)
				if c.ready {
					_ = c.close(false)
					c.eventMu.Unlock()

					return
				}
				_ = c.stop(false)
				c.driveMu.Unlock()
			}
		}
		changed := c.changed
		c.eventMu.Unlock()
		select {
		case <-changed:
		case <-events:
		}
	}
}

func (c *Conn) processReadyEvents(conn *dtls.DetachedConn) error {
	for {
		event := conn.NextEvent()
		var err error
		switch event.Kind {
		case dtls.DetachedNoEvent:
			return nil
		case dtls.DetachedWriteDatagrams:
			err = c.writeDatagrams(event.Datagrams)
		case dtls.DetachedApplicationData:
			err = c.Push(event.Data)
		case dtls.DetachedHandshakeDone:
			c.driveMu.Lock()
			c.ready = true
			c.notify(nil)
			c.driveMu.Unlock()
		case dtls.DetachedClosed:
			if event.Err == nil {
				event.Err = io.EOF
			}

			return event.Err
		}
		if err != nil {
			return err
		}
	}
}

func (c *Conn) writeDatagrams(datagrams [][]byte) error {
	for _, datagram := range datagrams {
		if _, err := c.config.WriteDatagram(datagram); err != nil {
			return err
		}
	}

	return nil
}

func (c *Conn) write(data []byte) (int, error) {
	c.driveMu.Lock()
	defer c.driveMu.Unlock()
	if c.ready {
		return c.dtlsConn.Write(data)
	}

	for {
		if c.closed {
			return 0, io.ErrClosedPipe
		}
		select {
		case <-c.writeDeadline.Done():
			return 0, os.ErrDeadlineExceeded
		default:
		}
		changed := c.changed
		c.driveMu.Unlock()
		select {
		case <-changed:
		case <-c.writeDeadline.Done():
		}
		c.driveMu.Lock()
		if c.ready {
			return c.dtlsConn.Write(data)
		}
	}
}

func (c *Conn) Close() error {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()
	c.driveMu.Lock()

	return c.close(true)
}

func (c *Conn) close(flushEvents bool) error {
	var onClose func()
	if !c.closed {
		c.closed = true
		c.closeErr = util.FlattenErrs([]error{c.stop(flushEvents), c.Conn.Close()})
		c.writeDeadline.Set(time.Time{})
		c.config.SetDatagramHandler(nil)
		onClose = c.config.OnClose
	}
	err := c.closeErr
	c.driveMu.Unlock()
	if onClose != nil {
		onClose()
	}

	return err
}

func (c *Conn) stop(flush bool) error {
	var errs []error
	if c.dtlsConn != nil {
		errs = append(errs, c.dtlsConn.Close())
		if flush {
			for event := c.dtlsConn.NextEvent(); event.Kind != dtls.DetachedNoEvent; event = c.dtlsConn.NextEvent() {
				if event.Kind == dtls.DetachedWriteDatagrams {
					errs = append(errs, c.writeDatagrams(event.Datagrams))
				}
			}
		}
	}
	c.dtlsConn, c.ready = nil, false
	c.notify(io.ErrClosedPipe)

	return util.FlattenErrs(errs)
}

func (c *Conn) notify(err error) {
	if c.handshakeDone != nil {
		c.handshakeDone <- err
		c.handshakeDone = nil
	}
	close(c.changed)
	c.changed = make(chan struct{})
}
