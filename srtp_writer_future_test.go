// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"io"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/srtp/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSWFStopClosed() *srtpWriterFuture {
	stop := make(chan struct{})
	close(stop)

	tr := &DTLSTransport{
		srtpReady: make(chan struct{}),
	}
	sender := &RTPSender{
		stopCalled: stop,
		transport:  tr,
	}

	return &srtpWriterFuture{
		ssrc:      1234,
		rtpSender: sender,
	}
}

func newSWFReadyButNoSessions() *srtpWriterFuture {
	tr := &DTLSTransport{
		srtpReady: make(chan struct{}),
	}
	close(tr.srtpReady)

	sender := &RTPSender{
		stopCalled: make(chan struct{}),
		transport:  tr,
	}

	return &srtpWriterFuture{
		ssrc:      5678,
		rtpSender: sender,
	}
}

func TestSRTPWriterFuture_Errors_WhenStopCalled(t *testing.T) {
	swf := newSWFStopClosed()

	n, err := swf.WriteRTP(&rtp.Header{}, []byte("x"))
	assert.Zero(t, n)
	assert.ErrorIs(t, err, io.ErrClosedPipe)

	n, err = swf.Write([]byte("x"))
	assert.Zero(t, n)
	assert.ErrorIs(t, err, io.ErrClosedPipe)

	buf := make([]byte, 1)
	n, err = swf.Read(buf)
	assert.Zero(t, n)
	assert.ErrorIs(t, err, io.ErrClosedPipe)

	err = swf.SetReadDeadline(time.Now())
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestSRTPWriterFuture_Errors_WhenClosedFlagSet(t *testing.T) {
	tr := &DTLSTransport{srtpReady: make(chan struct{})}
	close(tr.srtpReady)

	sender := &RTPSender{
		stopCalled: make(chan struct{}),
		transport:  tr,
	}

	swf := &srtpWriterFuture{
		ssrc:      42,
		rtpSender: sender,
		closed:    true,
	}

	_, err := swf.WriteRTP(&rtp.Header{}, nil)
	assert.ErrorIs(t, err, io.ErrClosedPipe)

	_, err = swf.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.ErrClosedPipe)

	err = swf.SetReadDeadline(time.Now())
	assert.ErrorIs(t, err, io.ErrClosedPipe)

	_, err = swf.Write(nil)
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestSRTPWriterFuture_Errors_WhenSessionsUnavailable(t *testing.T) {
	swf := newSWFReadyButNoSessions()

	n, err := swf.WriteRTP(&rtp.Header{}, nil)
	assert.Zero(t, n)
	require.Error(t, err)

	n, err = swf.Write([]byte("data"))
	assert.Zero(t, n)
	require.Error(t, err)

	n, err = swf.Read(make([]byte, 1))
	assert.Zero(t, n)
	require.Error(t, err)

	err = swf.SetReadDeadline(time.Now())
	require.Error(t, err)
}

func TestSRTPWriterFuture_Close_AlreadyClosed(t *testing.T) {
	s := &srtpWriterFuture{
		closed: true,
	}
	s.rtcpReadStream.Store(&srtp.ReadStreamSRTCP{})

	err := s.Close()
	assert.NoError(t, err, "Close on an already-closed srtpWriterFuture should return nil")
}
