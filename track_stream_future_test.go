// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/srtp/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTSFReadyButNoSessions(t *testing.T) *trackStreamFuture {
	t.Helper()
	ready, cancel := context.WithCancel(t.Context())
	cancel()

	return &trackStreamFuture{transport: &DTLSTransport{}, ready: ready.Done()}
}

func TestTrackStreamFuture_Errors_WhenStopCalled(t *testing.T) {
	stopped, stop := context.WithCancel(t.Context())
	future := &trackStreamFuture{stopped: stopped.Done(), stopErr: io.ErrClosedPipe}

	// Writes before SRTP is ready must return without blocking.
	n, err := future.WriteRTP(&rtp.Header{}, []byte("x"))
	assert.Zero(t, n)
	assert.NoError(t, err)

	stop()
	n, err = future.WriteRTP(&rtp.Header{}, []byte("x"))
	assert.Zero(t, n)
	assert.ErrorIs(t, err, io.ErrClosedPipe)

	buf := make([]byte, 1)
	n, err = future.Read(buf)
	assert.Zero(t, n)
	assert.ErrorIs(t, err, io.ErrClosedPipe)

	err = future.SetReadDeadline(time.Now())
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestTrackStreamFuture_Errors_WhenClosedFlagSet(t *testing.T) {
	future := newTSFReadyButNoSessions(t)
	future.closed = true

	_, err := future.WriteRTP(&rtp.Header{}, nil)
	assert.ErrorIs(t, err, io.ErrClosedPipe)

	_, err = future.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.ErrClosedPipe)

	err = future.SetReadDeadline(time.Now())
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestTrackStreamFuture_Errors_WhenSessionsUnavailable(t *testing.T) {
	future := newTSFReadyButNoSessions(t)

	n, err := future.WriteRTP(&rtp.Header{}, nil)
	assert.Zero(t, n)
	require.Error(t, err)

	n, err = future.Read(make([]byte, 1))
	assert.Zero(t, n)
	require.Error(t, err)

	err = future.SetReadDeadline(time.Now())
	require.Error(t, err)
}

func TestTrackStreamFuture_Close_AlreadyClosed(t *testing.T) {
	s := &trackStreamFuture{
		closed: true,
	}
	s.rtcpReadStream.Store(&srtp.ReadStreamSRTCP{})

	err := s.Close()
	assert.NoError(t, err, "Close on an already-closed trackStreamFuture should return nil")
}
