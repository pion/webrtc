package mux

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/pion/transport/connctx"
	"github.com/pion/transport/test"
)

func TestStressDuplex(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Check for leaking routines
	report := test.CheckRoutines(t)
	defer report()

	// Run the test
	stressDuplex(ctx, t)
}

func stressDuplex(ctx context.Context, t *testing.T) {
	ca, cb, stop := pipeMemory(ctx)

	defer func() {
		stop(t)
	}()

	opt := test.Options{
		MsgSize:  2048,
		MsgCount: 100,
	}

	t.Run("WithoutContext", func(t *testing.T) {
		err := test.StressDuplex(ca, cb.Conn(), opt)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("WithContext", func(t *testing.T) {
		err := test.StressDuplexContext(context.Background(), ca, cb, opt)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func pipeMemory(ctx context.Context) (*Endpoint, connctx.ConnCtx, func(*testing.T)) {
	// In memory pipe
	ca, cb := net.Pipe()

	matchAll := func([]byte) bool {
		return true
	}

	config := Config{
		Conn:          connctx.New(ca),
		BufferSize:    8192,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	}

	m := NewMux(ctx, config)
	e := m.NewEndpoint(matchAll)
	m.RemoveEndpoint(e)
	e = m.NewEndpoint(matchAll)

	stop := func(t *testing.T) {
		err := cb.Close()
		if err != nil {
			t.Fatal(err)
		}
		err = m.Close()
		if err != nil {
			t.Fatal(err)
		}
	}

	return e, connctx.New(cb), stop
}

func TestNoEndpoints(t *testing.T) {
	// In memory pipe
	ca, cb := net.Pipe()
	err := cb.Close()
	if err != nil {
		panic("Failed to close network pipe")
	}

	config := Config{
		Conn:          connctx.New(ca),
		BufferSize:    8192,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	m := NewMux(ctx, config)
	err = m.dispatch(ctx, make([]byte, 1))
	if err != nil {
		t.Fatal(err)
	}
	err = m.Close()
	if err != nil {
		t.Fatalf("Failed to close empty mux")
	}
	err = ca.Close()
	if err != nil {
		panic("Failed to close network pipe")
	}
}
