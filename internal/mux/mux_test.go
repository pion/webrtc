// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package mux

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/pion/transport/v2/packetio"
	"github.com/pion/transport/v2/test"
	"github.com/stretchr/testify/require"
)

const testPipeBufferSize = 8192

func TestNoEndpoints(t *testing.T) {
	// In memory pipe
	ca, cb := net.Pipe()
	require.NoError(t, cb.Close())

	m := NewMux(Config{
		Conn:          ca,
		BufferSize:    testPipeBufferSize,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	require.NoError(t, m.dispatch(make([]byte, 1)))
	require.NoError(t, m.Close())
	require.NoError(t, ca.Close())
}

type muxErrorConnReadResult struct {
	err  error
	data []byte
}

// muxErrorConn
type muxErrorConn struct {
	net.Conn
	readResults []muxErrorConnReadResult
}

func (m *muxErrorConn) Read(b []byte) (n int, err error) {
	err = m.readResults[0].err
	copy(b, m.readResults[0].data)
	n = len(m.readResults[0].data)

	m.readResults = m.readResults[1:]
	return
}

/*
Don't end the mux readLoop for packetio.ErrTimeout or io.ErrShortBuffer, assert the following

  - io.ErrShortBuffer and packetio.ErrTimeout don't end the read loop

  - io.EOF ends the loop

    pion/webrtc#1720
*/
func TestNonFatalRead(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	expectedData := []byte("expectedData")

	// In memory pipe
	ca, cb := net.Pipe()
	require.NoError(t, cb.Close())

	conn := &muxErrorConn{ca, []muxErrorConnReadResult{
		// Non-fatal timeout error
		{packetio.ErrTimeout, nil},
		{nil, expectedData},
		{io.ErrShortBuffer, nil},
		{nil, expectedData},
		{io.EOF, nil},
	}}

	m := NewMux(Config{
		Conn:          conn,
		BufferSize:    testPipeBufferSize,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})

	e := m.NewEndpoint(MatchAll)

	buff := make([]byte, testPipeBufferSize)
	n, err := e.Read(buff)
	require.NoError(t, err)
	require.Equal(t, buff[:n], expectedData)

	n, err = e.Read(buff)
	require.NoError(t, err)
	require.Equal(t, buff[:n], expectedData)

	<-m.closedCh
	require.NoError(t, m.Close())
	require.NoError(t, ca.Close())
}

// If a endpoint returns packetio.ErrFull it is a non-fatal error and shouldn't cause
// the mux to be destroyed
// pion/webrtc#2180
func TestNonFatalDispatch(t *testing.T) {
	in, out := net.Pipe()

	m := NewMux(Config{
		Conn:          out,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
		BufferSize:    1500,
	})

	e := m.NewEndpoint(MatchSRTP)
	e.buffer.SetLimitSize(1)

	for i := 0; i <= 25; i++ {
		srtpPacket := []byte{128, 1, 2, 3, 4}
		_, err := in.Write(srtpPacket)
		require.NoError(t, err)
	}

	require.NoError(t, m.Close())
	require.NoError(t, in.Close())
	require.NoError(t, out.Close())
}

func BenchmarkDispatch(b *testing.B) {
	m := &Mux{
		endpoints: make(map[*Endpoint]MatchFunc),
		log:       logging.NewDefaultLoggerFactory().NewLogger("mux"),
	}

	e := m.NewEndpoint(MatchSRTP)
	m.NewEndpoint(MatchSRTCP)

	buf := []byte{128, 1, 2, 3, 4}
	buf2 := make([]byte, 1200)

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		err := m.dispatch(buf)
		if err != nil {
			b.Errorf("dispatch: %v", err)
		}
		_, err = e.buffer.Read(buf2)
		if err != nil {
			b.Errorf("read: %v", err)
		}
	}
}
