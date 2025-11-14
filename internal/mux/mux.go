// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package mux multiplexes packets on a single socket (RFC7983)
package mux

import (
	"errors"
	"io"
	"sync"

	"github.com/pion/ice/v4"
	"github.com/pion/logging"
	"github.com/pion/transport/v3"
	"github.com/pion/transport/v3/packetio"
)

const (
	// The maximum amount of data that can be buffered before returning errors.
	maxBufferSize = 1000 * 1000 // 1MB

	// How many total pending packets can be cached.
	maxPendingPackets = 15
)

// Config collects the arguments to mux.Mux construction into
// a single structure.
type Config struct {
	Conn          transport.NetConnSocket
	BufferSize    int
	LoggerFactory logging.LoggerFactory
}

type pendingPacket struct {
	packet []byte
	attr   *transport.PacketAttributes
}

// Mux allows multiplexing.
type Mux struct {
	nextConn   transport.NetConnSocket
	bufferSize int
	lock       sync.Mutex
	endpoints  map[*Endpoint]MatchFunc
	isClosed   bool

	pendingPackets []*pendingPacket

	closedCh chan struct{}
	log      logging.LeveledLogger
}

// NewMux creates a new Mux.
func NewMux(config Config) *Mux {
	mux := &Mux{
		nextConn:   config.Conn,
		endpoints:  make(map[*Endpoint]MatchFunc),
		bufferSize: config.BufferSize,
		closedCh:   make(chan struct{}),
		log:        config.LoggerFactory.NewLogger("mux"),
	}

	go mux.readLoop()

	return mux
}

// NewEndpoint creates a new Endpoint.
func (m *Mux) NewEndpoint(matchFunc MatchFunc) *Endpoint {
	endpoint := &Endpoint{
		mux:    m,
		buffer: packetio.NewBuffer(),
	}

	// Set a maximum size of the buffer in bytes.
	endpoint.buffer.SetLimitSize(maxBufferSize)

	m.lock.Lock()
	m.endpoints[endpoint] = matchFunc
	m.lock.Unlock()

	go m.handlePendingPackets(endpoint, matchFunc)

	return endpoint
}

// RemoveEndpoint removes an endpoint from the Mux.
func (m *Mux) RemoveEndpoint(e *Endpoint) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.endpoints, e)
}

// Close closes the Mux and all associated Endpoints.
func (m *Mux) Close() error {
	m.lock.Lock()
	for e := range m.endpoints {
		if err := e.close(); err != nil {
			m.lock.Unlock()

			return err
		}

		delete(m.endpoints, e)
	}
	m.isClosed = true
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
	attr := transport.NewPacketAttributesWithLen(transport.MaxAttributesLen)
	for {
		n, err := m.nextConn.ReadWithAttributes(buf, attr)
		switch {
		case errors.Is(err, io.EOF), errors.Is(err, ice.ErrClosed):
			return
		case errors.Is(err, io.ErrShortBuffer), errors.Is(err, packetio.ErrTimeout):
			m.log.Errorf("mux: failed to read from packetio.Buffer %s", err.Error())

			continue
		case err != nil:
			m.log.Errorf("mux: ending readLoop packetio.Buffer error %s", err.Error())

			return
		}

		if err = m.dispatch(buf[:n], attr.GetReadPacketAttributes()); err != nil {
			if errors.Is(err, io.ErrClosedPipe) {
				// if the buffer was closed, that's not an error we care to report
				return
			}
			m.log.Errorf("mux: ending readLoop dispatch error %s", err.Error())

			return
		}
	}
}

func (m *Mux) dispatch(b []byte, attr *transport.PacketAttributes) error {
	if len(b) == 0 {
		m.log.Warnf("Warning: mux: unable to dispatch zero length packet")

		return nil
	}

	var endpoint *Endpoint

	m.lock.Lock()
	for e, f := range m.endpoints {
		if f(b) {
			endpoint = e

			break
		}
	}
	if endpoint == nil {
		defer m.lock.Unlock()

		if !m.isClosed {
			if len(m.pendingPackets) >= maxPendingPackets {
				m.log.Warnf(
					"Warning: mux: no endpoint for packet starting with %d, not adding to queue size(%d)",
					b[0], //nolint:gosec // G602, false positive?
					len(m.pendingPackets),
				)
			} else {
				m.log.Warnf(
					"Warning: mux: no endpoint for packet starting with %d, adding to queue size(%d)",
					b[0], //nolint:gosec // G602, false positive?
					len(m.pendingPackets),
				)
				// copy the packet bytes and clone the PacketAttributes
				pp := &pendingPacket{
					packet: append([]byte{}, b...),
					attr:   attr.Clone(),
				}

				m.pendingPackets = append(m.pendingPackets, pp)
			}
		}

		return nil
	}

	m.lock.Unlock()
	_, err := endpoint.buffer.WriteWithAttributes(b, attr)

	// Expected when bytes are received faster than the endpoint can process them (#2152, #2180)
	if errors.Is(err, packetio.ErrFull) {
		m.log.Infof("mux: endpoint buffer is full, dropping packet")

		return nil
	}

	return err
}

func (m *Mux) handlePendingPackets(endpoint *Endpoint, matchFunc MatchFunc) {
	m.lock.Lock()
	defer m.lock.Unlock()

	pendingPackets := make([]*pendingPacket, len(m.pendingPackets))
	for _, p := range m.pendingPackets {
		if matchFunc(p.packet) {
			if _, err := endpoint.buffer.WriteWithAttributes(p.packet, p.attr); err != nil {
				m.log.Warnf("Warning: mux: error writing packet to endpoint from pending queue: %s", err)
			}
		} else {
			pendingPackets = append(pendingPackets, p) //nolint:makezero // todo fix
		}
	}
	m.pendingPackets = pendingPackets
}
