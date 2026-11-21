// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"errors"
	"io"
	"net"
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
//
// When rtxSSRC is set, RTCP for the track arrives on two SRTCP read streams:
// one for the media SSRC and one for the RTX SSRC. A receiver puts the SSRC
// of the last packet it received into its transport-wide congestion control
// feedback, which after a retransmission or RTX padding is the RTX SSRC, so
// feedback addressed to it must reach the sender too. Read merges both
// streams; SetReadDeadline applies to the media stream.
type trackStreamFuture struct {
	ssrc              SSRC
	rtxSSRC           SSRC
	transport         *DTLSTransport
	ready, stopped    <-chan struct{}
	stopErr           error
	rtcpReadStream    atomic.Pointer[srtp.ReadStreamSRTCP]
	rtcpRTXReadStream atomic.Pointer[srtp.ReadStreamSRTCP]
	rtpWriteStream    atomic.Pointer[srtp.WriteStreamSRTP]
	mu                sync.Mutex
	closed            bool

	// rtcpReads carries the packets of both RTCP read streams when RTX is
	// negotiated; rtcpClosed is closed by Close so a pending Read returns.
	rtcpReads  chan srtcpRead
	rtcpClosed chan struct{}
}

type srtcpRead struct {
	buf []byte
	err error
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
	// Sending needs SRTP even when no SDP negotiation starts it (the ORTC path).
	if err := s.transport.wantSRTP(); err != nil {
		return err
	}
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
	if s.rtcpReadStream.Load() != nil {
		return nil
	}

	srtcpSession, err := s.transport.getSRTCPSession()
	if err != nil {
		return err
	}

	rtcpReadStream, err := srtcpSession.OpenReadStream(uint32(s.ssrc))
	if err != nil {
		return err
	}

	var rtcpRTXReadStream *srtp.ReadStreamSRTCP
	if s.rtxSSRC != 0 {
		rtcpRTXReadStream, err = srtcpSession.OpenReadStream(uint32(s.rtxSSRC))
		if err != nil {
			return err
		}
	}

	srtpSession, err := s.transport.getSRTPSession()
	if err != nil {
		return err
	}

	rtpWriteStream, err := srtpSession.OpenWriteStream()
	if err != nil {
		return err
	}

	if rtcpRTXReadStream != nil {
		s.rtcpReads = make(chan srtcpRead)
		s.rtcpClosed = make(chan struct{})
		s.rtcpRTXReadStream.Store(rtcpRTXReadStream)
		go s.readRTCPInto(rtcpReadStream, true)
		go s.readRTCPInto(rtcpRTXReadStream, false)
	}
	s.rtcpReadStream.Store(rtcpReadStream)
	s.rtpWriteStream.Store(rtpWriteStream)

	return nil
}

// readRTCPInto forwards every packet of stream to rtcpReads until the
// stream is closed. Only the media stream forwards errors: a deadline set
// through SetReadDeadline applies to it alone, and the RTX stream has no
// readers of its own.
func (s *trackStreamFuture) readRTCPInto(stream *srtp.ReadStreamSRTCP, forwardErrors bool) {
	for {
		buf := make([]byte, s.transport.api.settingEngine.getReceiveMTU())
		n, err := stream.Read(buf)
		if err != nil {
			if !forwardErrors {
				if isSRTCPStreamClosed(err) {
					return
				}

				continue
			}
			select {
			case s.rtcpReads <- srtcpRead{err: err}:
			case <-s.rtcpClosed:
				return
			}
			if isSRTCPStreamClosed(err) {
				return
			}

			continue
		}
		select {
		case s.rtcpReads <- srtcpRead{buf: buf[:n]}:
		case <-s.rtcpClosed:
			return
		}
	}
}

// isSRTCPStreamClosed reports whether a read error ends the stream rather
// than one Read (a deadline).
func isSRTCPStreamClosed(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}

	return true
}

func (s *trackStreamFuture) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	if s.rtcpClosed != nil {
		close(s.rtcpClosed)
	}
	var err error
	if value := s.rtcpRTXReadStream.Load(); value != nil {
		err = value.Close()
	}
	if value := s.rtcpReadStream.Load(); value != nil {
		if closeErr := value.Close(); closeErr != nil {
			err = closeErr
		}
	}

	return err
}

func (s *trackStreamFuture) Read(b []byte) (n int, err error) {
	if s.rtcpReadStream.Load() == nil {
		if err := s.init(false); err != nil {
			return 0, err
		}
	}

	if s.rtcpRTXReadStream.Load() == nil {
		return s.rtcpReadStream.Load().Read(b)
	}

	return s.readMerged(b)
}

// readMerged returns the next packet from either RTCP read stream.
func (s *trackStreamFuture) readMerged(b []byte) (int, error) {
	select {
	case r := <-s.rtcpReads:
		if r.err != nil {
			return 0, r.err
		}
		if len(b) < len(r.buf) {
			return 0, io.ErrShortBuffer
		}

		return copy(b, r.buf), nil
	case <-s.rtcpClosed:
		return 0, io.EOF
	}
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
