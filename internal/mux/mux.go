// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package mux multiplexes packets on a single socket (RFC7983)
package mux

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/pion/ice/v3"
	"github.com/pion/logging"
	"github.com/pion/transport/v3/packetio"
)

// The maximum amount of data that can be buffered before returning errors.
const maxBufferSize = 1000 * 1000 // 1MB

// Config collects the arguments to mux.Mux construction into
// a single structure
type Config struct {
	Conn          net.Conn
	BufferSize    int
	LoggerFactory logging.LoggerFactory
}

// Mux allows multiplexing
type Mux struct {
	lock       sync.RWMutex
	nextConn   net.Conn
	endpoints  map[*Endpoint]MatchFunc
	bufferSize int
	closedCh   chan struct{}

	pendingPackets     map[*pendingPacket]struct{}
	pendingPacketsLock sync.Mutex

	log logging.LeveledLogger
}

// NewMux creates a new Mux
func NewMux(config Config) *Mux {
	m := &Mux{
		nextConn:       config.Conn,
		endpoints:      make(map[*Endpoint]MatchFunc),
		bufferSize:     config.BufferSize,
		closedCh:       make(chan struct{}),
		log:            config.LoggerFactory.NewLogger("mux"),
		pendingPackets: make(map[*pendingPacket]struct{}),
	}

	go m.readLoop()
	go m.pendingPacketsHandler()

	return m
}

// NewEndpoint creates a new Endpoint
func (m *Mux) NewEndpoint(f MatchFunc) *Endpoint {
	e := &Endpoint{
		mux:    m,
		buffer: packetio.NewBuffer(),
	}

	// Set a maximum size of the buffer in bytes.
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
		if err := e.close(); err != nil {
			m.lock.Unlock()
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

		if err = m.dispatch(buf[:n]); err != nil {
			m.log.Errorf("mux: ending readLoop dispatch error %s", err.Error())
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
		if len(buf) > 0 {
			m.log.Warnf("Warning: mux: no endpoint for packet starting with %d, queueing packet as pending", buf[0])
		} else {
			m.log.Warnf("Warning: mux: no endpoint for zero length packet, queueing packet as pending")
		}

		m.pendingPacketsLock.Lock()
		m.pendingPackets[&pendingPacket{
			t:    time.Now(),
			data: buf,
		}] = struct{}{}
		m.pendingPacketsLock.Unlock()
		return nil
	}

	_, err := endpoint.buffer.Write(buf)

	// Expected when bytes are received faster than the endpoint can process them (#2152, #2180)
	if errors.Is(err, packetio.ErrFull) {
		m.log.Infof("mux: endpoint buffer is full, dropping packet")
		return nil
	}

	return err
}

func (m *Mux) pendingPacketsHandler() {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.lock.RLock()
			m.pendingPacketsLock.Lock()
			for p := range m.pendingPackets {
				if time.Since(p.t) > time.Second*5 {
					m.log.Warnf("Warning: mux: dropping packet after 5 seconds in pending queue")
					delete(m.pendingPackets, p)
				}
				for endpoint, f := range m.endpoints {
					if f(p.data) {
						_, _ = endpoint.buffer.Write(p.data)
						delete(m.pendingPackets, p)
						m.log.Warnf("Warning: mux: found endpoint for packet after %s in pending queue", time.Since(p.t))
					}
				}
			}
			m.pendingPacketsLock.Unlock()
			m.lock.RUnlock()
		case <-m.closedCh:
			return
		}
	}
}

type pendingPacket struct {
	t    time.Time
	data []byte
}
