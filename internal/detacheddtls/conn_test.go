// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package detacheddtls

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/pion/dtls/v4"
	"github.com/pion/dtls/v4/pkg/crypto/selfsign"
	"github.com/pion/dtls/v4/pkg/protocol"
	"github.com/pion/webrtc/v5/internal/netconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// peerWithPiggybacking is a Conn whose handshake flights are piggybacked
// instead of written to the network, standing in for the ICE agent.
type peerWithPiggybacking struct {
	conn *Conn

	mu          sync.Mutex
	handler     func([]byte) error
	piggybacked [][]byte
	written     chan []byte
	accept      bool
}

func newPeerWithPiggybacking(t *testing.T, accept bool) *peerWithPiggybacking {
	t.Helper()

	peer := &peerWithPiggybacking{written: make(chan []byte, 16), accept: accept}
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5000}
	closed := make(chan struct{})
	peer.conn = New(Config{
		WriteDatagram: func(datagram []byte) (int, error) {
			peer.written <- append([]byte(nil), datagram...)

			return len(datagram), nil
		},
		SetDatagramHandler: func(handler func([]byte) error) func() {
			peer.mu.Lock()
			peer.handler = handler
			peer.mu.Unlock()

			return func() {}
		},
		TransportClosed: closed,
		NetConn: netconn.Config{
			LocalAddr:        func() net.Addr { return addr },
			RemoteAddr:       func() net.Addr { return addr },
			SetWriteDeadline: func(time.Time) error { return nil },
		},
		Piggyback: func(datagrams [][]byte) bool {
			if !peer.accept {
				return false
			}
			peer.mu.Lock()
			defer peer.mu.Unlock()
			// Like the ICE agent a flight replaces the previous one.
			peer.piggybacked = peer.piggybacked[:0]
			for _, datagram := range datagrams {
				peer.piggybacked = append(peer.piggybacked, append([]byte(nil), datagram...))
			}

			return true
		},
		OnHandshakeDone: func(*dtls.DetachedConn) {},
	})
	t.Cleanup(func() {
		_ = peer.conn.Close()
		close(closed)
	})

	return peer
}

// takePiggybacked returns and clears the flight handed to Piggyback.
func (p *peerWithPiggybacking) takePiggybacked() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	piggybacked := p.piggybacked
	p.piggybacked = nil

	return piggybacked
}

// deliver hands datagrams to the peer the way the ICE agent's task loop does.
func (p *peerWithPiggybacking) deliver(t *testing.T, datagrams [][]byte) {
	t.Helper()
	require.Eventually(t, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()

		return p.handler != nil
	}, time.Second, time.Millisecond)
	p.mu.Lock()
	handler := p.handler
	p.mu.Unlock()
	for _, datagram := range datagrams {
		require.NoError(t, handler(datagram))
	}
}

func detachedPair(t *testing.T) (client, server *dtls.DetachedConn) {
	t.Helper()

	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5000}
	newOpts := func() []dtls.Option {
		cert, err := selfsign.GenerateSelfSigned()
		require.NoError(t, err)

		return []dtls.Option{
			dtls.WithCertificates(cert),
			dtls.WithInsecureSkipVerify(true),
			dtls.WithMinVersion(protocol.Version1_3),
			dtls.WithMaxVersion(protocol.Version1_3),
			dtls.WithFlightInterval(time.Minute),
		}
	}
	clientOpts := []dtls.ClientOption{}
	for _, opt := range newOpts() {
		clientOpts = append(clientOpts, opt)
	}
	serverOpts := []dtls.ServerOption{
		dtls.WithInsecureSkipVerifyHello(true),
		dtls.WithClientAuth(dtls.RequireAnyClientCert),
	}
	for _, opt := range newOpts() {
		serverOpts = append(serverOpts, opt)
	}

	client, err := dtls.DetachedClient(addr, clientOpts...)
	require.NoError(t, err)
	server, err = dtls.DetachedServer(addr, serverOpts...)
	require.NoError(t, err)

	return client, server
}

func start(conn *Conn, dtlsConn *dtls.DetachedConn) <-chan error {
	done := make(chan error, 1)
	go func() { done <- conn.Start(context.Background(), dtlsConn) }()

	return done
}

func TestSpedReplyIsPiggybackedBeforeHandleDatagramReturns(t *testing.T) {
	client, server := newPeerWithPiggybacking(t, true), newPeerWithPiggybacking(t, true)
	clientConn, serverConn := detachedPair(t)
	clientDone, serverDone := start(client.conn, clientConn), start(server.conn, serverConn)

	var clientHello [][]byte
	require.Eventually(t, func() bool {
		clientHello = client.takePiggybacked()

		return len(clientHello) > 0
	}, time.Second, time.Millisecond)

	server.deliver(t, clientHello)
	serverHello := server.takePiggybacked()
	require.NotEmpty(t, serverHello, "ServerHello must be ready for the binding response")

	client.deliver(t, serverHello)
	clientFinished := client.takePiggybacked()
	require.NotEmpty(t, clientFinished, "client Finished must be ready for the binding response")

	server.deliver(t, clientFinished)
	require.NoError(t, <-serverDone)
	require.Eventually(t, func() bool {
		ack := server.takePiggybacked()
		if len(ack) > 0 {
			client.deliver(t, ack)
		}

		return len(ack) > 0
	}, time.Second, time.Millisecond)
	require.NoError(t, <-clientDone)
}

func TestSpedDeclinedFlightOnTaskLoopIsWritten(t *testing.T) {
	client, server := newPeerWithPiggybacking(t, true), newPeerWithPiggybacking(t, false)
	clientConn, serverConn := detachedPair(t)
	start(client.conn, clientConn)
	start(server.conn, serverConn)

	var clientHello [][]byte
	require.Eventually(t, func() bool {
		clientHello = client.takePiggybacked()

		return len(clientHello) > 0
	}, time.Second, time.Millisecond)
	server.deliver(t, clientHello)

	select {
	case datagram := <-server.written:
		assert.NotEmpty(t, datagram)
	case <-time.After(time.Second):
		assert.Fail(t, "declined ServerHello was not written")
	}
}
