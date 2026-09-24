// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/srtp/v3"
	"github.com/pion/transport/v5/deadline"
)

// trackStreamFuture waits for a track's streams to become available.
// Senders wait for SRTP; receivers wait for RID binding.
type trackStreamFuture struct {
	ssrc           SSRC
	transport      *DTLSTransport
	ready, stopped <-chan struct{}
	stopErr        error
	rtcpReadStream atomic.Pointer[srtp.ReadStreamSRTCP]
	rtpWriteStream atomic.Pointer[srtp.WriteStreamSRTP]
	mu             sync.Mutex
	closed         bool
}

// wait blocks until ready, stopped, or the read deadline expires.
// A nil future is already ready; a nil deadline waits indefinitely.
func (s *trackStreamFuture) wait(readDeadline *deadline.Deadline) error {
	if s == nil {
		return nil
	}

	// Once ready, the underlying stream handles deadlines.
	select {
	case <-s.ready:
	default:
		var expired <-chan struct{}
		if readDeadline != nil {
			expired = readDeadline.Context().Done()
		}
		select {
		case <-s.stopped:
			return s.stopErr
		case <-expired:
			return os.ErrDeadlineExceeded
		case <-s.ready:
		}
	}

	select {
	case <-s.stopped:
		return s.stopErr
	default:
		return nil
	}
}

func (s *trackStreamFuture) init(returnWhenNoSRTP bool) error { //nolint:cyclop
	if returnWhenNoSRTP {
		select {
		case <-s.stopped:
			return io.ErrClosedPipe
		case <-s.ready:
		default:
			return nil
		}
	} else if err := s.wait(nil); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return io.ErrClosedPipe
	}

	srtcpSession, err := s.transport.getSRTCPSession()
	if err != nil {
		return err
	}

	rtcpReadStream, err := srtcpSession.OpenReadStream(uint32(s.ssrc))
	if err != nil {
		return err
	}

	srtpSession, err := s.transport.getSRTPSession()
	if err != nil {
		return err
	}

	rtpWriteStream, err := srtpSession.OpenWriteStream()
	if err != nil {
		return err
	}

	s.rtcpReadStream.Store(rtcpReadStream)
	s.rtpWriteStream.Store(rtpWriteStream)

	return nil
}

func (s *trackStreamFuture) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	if value := s.rtcpReadStream.Load(); value != nil {
		return value.Close()
	}

	return nil
}

func (s *trackStreamFuture) Read(b []byte) (n int, err error) {
	if s.rtcpReadStream.Load() == nil {
		if err := s.init(false); err != nil {
			return 0, err
		}
	}

	return s.rtcpReadStream.Load().Read(b)
}

func (s *trackStreamFuture) SetReadDeadline(t time.Time) error {
	if s.rtcpReadStream.Load() == nil {
		if err := s.init(false); err != nil {
			return err
		}
	}

	return s.rtcpReadStream.Load().SetReadDeadline(t)
}

func (s *trackStreamFuture) WriteRTP(header *rtp.Header, payload []byte) (int, error) {
	if s.rtpWriteStream.Load() == nil {
		if err := s.init(true); err != nil || s.rtpWriteStream.Load() == nil {
			return 0, err
		}
	}

	return s.rtpWriteStream.Load().WriteRTP(header, payload)
}
