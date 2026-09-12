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

	"github.com/pion/ice/v4"
	"github.com/pion/transport/v4/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestICETransport_StartContextClosedDuringDial is a regression test for
// ICETransport.StartContext returning errICETransportClosed without
// releasing the connection established by a successful Dial/Accept.
//
// Run with -race.
func TestICETransport_StartContextClosedDuringDial(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	err := runClosedDuringDial(t, nil)
	require.ErrorIs(t, err, errICETransportClosed)
}

// TestICETransport_StartContextClosedDuringDialGathererCallbackReentry is a
// regression test for the transport deadlocking when the gatherer is closed
// while the transport lock is held: the gatherer's OnStateChange callback
// runs synchronously, and if it re-enters the transport it must not block
// forever on the transport lock.
//
// Run with -race.
func TestICETransport_StartContextClosedDuringDialGathererCallbackReentry(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	err := runClosedDuringDial(t, func(transportA *ICETransport, gathererA *ICEGatherer) {
		gathererA.OnStateChange(func(s ICEGathererState) {
			if s == ICEGathererStateClosed {
				// Re-enter the transport from the gatherer callback.
				// This must not deadlock on the transport lock.
				_ = transportA.Start(nil, ICEParameters{}, nil) //nolint:errcheck
			}
		})
	})
	require.ErrorIs(t, err, errICETransportClosed)
}

// runClosedDuringDial deterministically drives StartContext into the
// "transport closed while Dial was in flight" branch:
//
//  1. Candidates are gathered but withheld, so the Dial cannot complete.
//  2. The transport is marked Closed while StartContext is inside Dial.
//  3. While holding the transport lock, candidates are injected directly
//     into the agents (bypassing the transport lock), so the connection is
//     established while the transport is closed and the Dial goroutine is
//     blocked trying to reacquire the transport lock.
//  4. The Connected state change is fully dispatched before the state is
//     forced back to Closed and the lock released, so the Dial goroutine
//     deterministically observes ICETransportStateClosed.
//
// If hook is non-nil it is invoked with transportA and gathererA before
// the connection is established, so callers can register extra callbacks.
// It returns the error produced by the local StartContext.
func runClosedDuringDial(
	t *testing.T,
	hook func(transportA *ICETransport, gathererA *ICEGatherer),
) error {
	t.Helper()

	// Disable mDNS so that the test also works in restricted CI
	// environments (e.g. i386 containers) where multicast binding fails
	// and host candidates would otherwise never be reachable.
	settingEngine := SettingEngine{}
	settingEngine.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	api := NewAPI(WithSettingEngine(settingEngine))

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

	// Collect candidates without exchanging them, and signal when
	// gathering completes.
	var (
		onceA       sync.Once
		onceB       sync.Once
		candidatesA []*ICECandidate
		candidatesB []*ICECandidate
		gatherDoneA = make(chan struct{})
		gatherDoneB = make(chan struct{})
	)
	gathererA.OnLocalCandidate(func(c *ICECandidate) {
		if c != nil {
			candidatesA = append(candidatesA, c)

			return
		}
		onceA.Do(func() { close(gatherDoneA) })
	})
	gathererB.OnLocalCandidate(func(c *ICECandidate) {
		if c != nil {
			candidatesB = append(candidatesB, c)

			return
		}
		onceB.Do(func() { close(gatherDoneB) })
	})
	require.NoError(t, gathererA.Gather())
	require.NoError(t, gathererB.Gather())
	awaitGatheringComplete(t, gatherDoneA, "gathererA")
	awaitGatheringComplete(t, gatherDoneB, "gathererB")

	if hook != nil {
		hook(transportA, gathererA)
	}

	// Register the Connected handler before starting StartContext so the
	// event cannot be missed no matter how quickly the connection is
	// established. The internal handler sets the transport state before
	// invoking user handlers, so once this channel fires no pending
	// Connected state write can overwrite setState(Closed) below.
	var (
		connectedOnce sync.Once
		connectedCh   = make(chan struct{})
	)
	transportA.OnConnectionStateChange(func(s ICETransportState) {
		if s == ICETransportStateConnected {
			connectedOnce.Do(func() { close(connectedCh) })
		}
	})

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
	// closed underneath it. The Dial cannot complete yet because no remote
	// candidates have been injected, so this is race-free.
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

	// The transport lock is still held and StartContext is inside Dial.
	// Inject the candidates directly into the agents, bypassing the
	// transport lock, so the connection is established while the transport
	// is closed. The Dial goroutine is then blocked trying to reacquire
	// the transport lock.
	agentA := gathererA.getAgent()
	agentB := gathererB.getAgent()
	require.NotNil(t, agentA)
	require.NotNil(t, agentB)
	injectCandidates(t, agentA, candidatesB)
	injectCandidates(t, agentB, candidatesA)

	// Wait for the Connected state change to be fully dispatched, then
	// force the state back to Closed and release the lock, so the Dial
	// goroutine deterministically observes ICETransportStateClosed.
	completeClosedUnderLock(t, transportA, connectedCh)

	err = receiveWithTimeout(t, errChA, 10*time.Second, "StartContext did not return")
	require.ErrorIs(t, err, errICETransportClosed)

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

	return err
}

// injectCandidates converts and adds the candidates to the agent.
func injectCandidates(t *testing.T, agent *ice.Agent, candidates []*ICECandidate) {
	t.Helper()

	for _, c := range candidates {
		iceCandidate, err := c.ToICE()
		require.NoError(t, err)
		require.NoError(t, agent.AddRemoteCandidate(iceCandidate))
	}
}

// completeClosedUnderLock waits for the Connected state change to be
// fully dispatched, then forces the transport state back to Closed and
// releases the transport lock. The caller must hold the lock.
func completeClosedUnderLock(t *testing.T, transport *ICETransport, connectedCh <-chan struct{}) {
	t.Helper()

	select {
	case <-connectedCh:
	case <-time.After(10 * time.Second):
		transport.lock.Unlock()
		assert.FailNow(t, "ICE connection state change was not dispatched")
	}

	transport.setState(ICETransportStateClosed)
	transport.lock.Unlock()
}

// awaitGatheringComplete waits for the gathering-done signal.
func awaitGatheringComplete(t *testing.T, done <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		assert.FailNow(t, name+" gathering did not complete")
	}
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
