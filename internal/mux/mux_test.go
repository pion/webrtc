package mux

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/pion/transport/packetio"
	"github.com/pion/transport/test"
	"github.com/stretchr/testify/assert"
)

const testPipeBufferSize = 8192

func TestStressDuplex(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	// Check for leaking routines
	report := test.CheckRoutines(t)
	defer report()

	// Run the test
	stressDuplex(t)
}

func stressDuplex(t *testing.T) {
	ca, cb, stop := pipeMemory()

	defer func() {
		stop(t)
	}()

	opt := test.Options{
		MsgSize:  2048,
		MsgCount: 100,
	}

	assert.NoError(t, test.StressDuplex(ca, cb, opt))
}

func pipeMemory() (*Endpoint, net.Conn, func(*testing.T)) {
	// In memory pipe
	ca, cb := net.Pipe()

	m := NewMux(Config{
		Conn:          ca,
		BufferSize:    testPipeBufferSize,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})

	e := m.NewEndpoint(MatchAll)
	m.RemoveEndpoint(e)
	e = m.NewEndpoint(MatchAll)

	stop := func(t *testing.T) {
		assert.NoError(t, cb.Close())
		assert.NoError(t, m.Close())
	}

	return e, cb, stop
}

func TestNoEndpoints(t *testing.T) {
	// In memory pipe
	ca, cb := net.Pipe()
	assert.NoError(t, cb.Close())

	m := NewMux(Config{
		Conn:          ca,
		BufferSize:    testPipeBufferSize,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	assert.NoError(t, m.dispatch(make([]byte, 1)))
	assert.NoError(t, m.Close())
	assert.NoError(t, ca.Close())
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

/* Don't end the mux readLoop for packetio.ErrTimeout or io.ErrShortBuffer, assert the following
   * io.ErrShortBuffer and packetio.ErrTimeout don't end the read loop
   * io.EOF ends the loop

   pion/webrtc#1720
*/
func TestNonFatalRead(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	expectedData := []byte("expectedData")

	// In memory pipe
	ca, cb := net.Pipe()
	assert.NoError(t, cb.Close())

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
	assert.NoError(t, err)
	assert.Equal(t, buff[:n], expectedData)

	n, err = e.Read(buff)
	assert.NoError(t, err)
	assert.Equal(t, buff[:n], expectedData)

	<-m.closedCh
	assert.NoError(t, m.Close())
	assert.NoError(t, ca.Close())
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
