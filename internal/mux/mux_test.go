// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package mux

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/pion/transport/v3/packetio"
	"github.com/pion/transport/v3/test"
	"github.com/stretchr/testify/require"
)

const testPipeBufferSize = 8192

func TestNoEndpoints(t *testing.T) {
	// In memory pipe
	ca, cb := net.Pipe()
	require.NoError(t, cb.Close())

	mux := NewMux(Config{
		Conn:          ca,
		BufferSize:    testPipeBufferSize,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	require.NoError(t, mux.dispatch(make([]byte, 1)))
	require.NoError(t, mux.Close())
	require.NoError(t, ca.Close())
}

type muxErrorConnReadResult struct {
	err  error
	data []byte
}

// muxErrorConn.
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

	mux := NewMux(Config{
		Conn:          conn,
		BufferSize:    testPipeBufferSize,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})

	e := mux.NewEndpoint(MatchAll)

	buff := make([]byte, testPipeBufferSize)
	n, err := e.Read(buff)
	require.NoError(t, err)
	require.Equal(t, buff[:n], expectedData)

	n, err = e.Read(buff)
	require.NoError(t, err)
	require.Equal(t, buff[:n], expectedData)

	<-mux.closedCh
	require.NoError(t, mux.Close())
	require.NoError(t, ca.Close())
}

// If a endpoint returns packetio.ErrFull it is a non-fatal error and shouldn't cause
// the mux to be destroyed
// pion/webrtc#2180
// .
func TestNonFatalDispatch(t *testing.T) {
	in, out := net.Pipe()

	mux := NewMux(Config{
		Conn:          out,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
		BufferSize:    1500,
	})

	e := mux.NewEndpoint(MatchSRTP)
	e.buffer.SetLimitSize(1)

	for i := 0; i <= 25; i++ {
		srtpPacket := []byte{128, 1, 2, 3, 4}
		_, err := in.Write(srtpPacket)
		require.NoError(t, err)
	}

	require.NoError(t, mux.Close())
	require.NoError(t, in.Close())
	require.NoError(t, out.Close())
}

func BenchmarkDispatch(b *testing.B) {
	mux := &Mux{
		endpoints: make(map[*Endpoint]MatchFunc),
		log:       logging.NewDefaultLoggerFactory().NewLogger("mux"),
	}

	endpoint := mux.NewEndpoint(MatchSRTP)
	mux.NewEndpoint(MatchSRTCP)

	buf := []byte{128, 1, 2, 3, 4}
	buf2 := make([]byte, 1200)

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		err := mux.dispatch(buf)
		if err != nil {
			b.Errorf("dispatch: %v", err)
		}
		_, err = endpoint.buffer.Read(buf2)
		if err != nil {
			b.Errorf("read: %v", err)
		}
	}
}

func TestPendingQueue(t *testing.T) {
	factory := logging.NewDefaultLoggerFactory()
	factory.DefaultLogLevel = logging.LogLevelDebug
	mux := &Mux{
		endpoints: make(map[*Endpoint]MatchFunc),
		log:       factory.NewLogger("mux"),
	}

	// Assert empty packets don't end up in queue
	require.NoError(t, mux.dispatch([]byte{}))
	require.Equal(t, len(mux.pendingPackets), 0)

	// Test Happy Case
	inBuffer := []byte{20, 1, 2, 3, 4}
	outBuffer := make([]byte, len(inBuffer))

	require.NoError(t, mux.dispatch(inBuffer))

	endpoint := mux.NewEndpoint(MatchDTLS)
	require.NotNil(t, endpoint)

	_, err := endpoint.Read(outBuffer)
	require.NoError(t, err)

	require.Equal(t, outBuffer, inBuffer)

	// Assert limit on pendingPackets
	for i := 0; i <= 100; i++ {
		require.NoError(t, mux.dispatch([]byte{64, 65, 66}))
	}
	require.Equal(t, len(mux.pendingPackets), maxPendingPackets)
}
