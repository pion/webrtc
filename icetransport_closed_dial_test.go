// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/pion/transport/v4/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestICETransport_StartContextClosedDuringDial is a regression test for
// ICETransport.StartContext returning errICETransportClosed without
// releasing the connection established by a successful Dial/Accept.
//
// It deterministically drives StartContext into the "transport closed while
// Dial was in flight" branch by holding the transport lock while the ICE
// connection is established, so the Dial goroutine cannot observe any state
// other than Closed.
//
// Run with -race.
func TestICETransport_StartContextClosedDuringDial(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	api := NewAPI()
	gathererA, err := api.NewICEGatherer(ICEGatherOptions{})
	require.NoError(t, err)
	gathererB, err := api.NewICEGatherer(ICEGatherOptions{})
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, gathererA.Close())
		assert.NoError(t, gathererB.Close())
	}()

	transportA := api.NewICETransport(gathererA)
	transportB := api.NewICETransport(gathererB)

	paramsA, err := gathererA.GetLocalParameters()
	require.NoError(t, err)
	paramsB, err := gathererB.GetLocalParameters()
	require.NoError(t, err)

	// Exchange candidates gathered on each side.
	gathererA.OnLocalCandidate(func(c *ICECandidate) {
		if c != nil {
			_ = transportB.AddRemoteCandidate(c)
		}
	})
	gathererB.OnLocalCandidate(func(c *ICECandidate) {
		if c != nil {
			_ = transportA.AddRemoteCandidate(c)
		}
	})
	require.NoError(t, gathererA.Gather())
	require.NoError(t, gathererB.Gather())

	// Start the remote (controlled) side.
	controlled := ICERoleControlled
	errChB := make(chan error, 1)
	go func() {
		errChB <- transportB.StartContext(context.Background(), nil, paramsA, &controlled)
	}()
	defer func() {
		assert.NoError(t, transportB.Stop())
	}()

	// Start the local (controlling) side while holding the transport lock
	// so that StartContext blocks at its entry lock.
	transportA.lock.Lock()
	baseline := runtime.NumGoroutine()
	controlling := ICERoleControlling
	errChA := make(chan error, 1)
	go func() {
		errChA <- transportA.StartContext(context.Background(), nil, paramsB, &controlling)
	}()
	transportA.lock.Unlock()

	// Wait until StartContext has passed its entry checks and released the
	// lock to run the ICE Dial, then grab the lock and mark the transport
	// closed underneath it. The lock is intentionally kept held.
	require.Eventually(t, func() bool {
		if !transportA.lock.TryLock() {
			return false
		}

		if transportA.ctxCancel == nil {
			// StartContext has not released the entry lock yet.
			transportA.lock.Unlock()

			return false
		}

		transportA.setState(ICETransportStateClosed)

		return true
	}, 5*time.Second, time.Millisecond)

	// Wait until the connection is established. The state change callback
	// fires after the connected signal, so once it fires the Dial goroutine
	// is blocked trying to reacquire the transport lock (still held here).
	connectedCh := awaitConnected(t, transportA)
	select {
	case <-connectedCh:
	case <-time.After(10 * time.Second):
		transportA.lock.Unlock()
		assert.FailNow(t, "ICE connection was not established")
	}

	// The internal state callback has overwritten the state with Connected.
	// Force it back to Closed before releasing the lock, so the Dial
	// goroutine deterministically observes the Closed state.
	transportA.setState(ICETransportStateClosed)
	transportA.lock.Unlock()

	require.ErrorIs(t, receiveWithTimeout(t, errChA, 10*time.Second, "StartContext did not return"), errICETransportClosed)

	// The cancel function must be released, mirroring the error path.
	require.Nil(t, transportA.ctxCancel)

	// Tear down the remote side so its goroutines do not count against the
	// baseline, then verify the connection established by the Dial was
	// closed: its connectivity-check goroutines must wind down.
	assert.NoError(t, transportB.Stop())
	_ = receiveWithTimeout(t, errChB, 5*time.Second, "remote StartContext did not return after Stop")

	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() <= baseline
	}, 5*time.Second, 20*time.Millisecond)
}

// awaitConnected registers a one-shot handler on the transport that
// signals when the transport reaches the Connected state.
func awaitConnected(t *testing.T, transport *ICETransport) <-chan struct{} {
	t.Helper()

	var (
		once sync.Once
		ch   = make(chan struct{})
	)
	transport.OnConnectionStateChange(func(s ICETransportState) {
		if s == ICETransportStateConnected {
			once.Do(func() { close(ch) })
		}
	})

	return ch
}

// receiveWithTimeout waits for a value on ch, failing the test if the
// timeout expires first.
func receiveWithTimeout(t *testing.T, ch <-chan error, timeout time.Duration, msg string) error {
	t.Helper()

	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		assert.FailNow(t, msg)
	}

	return nil
}
