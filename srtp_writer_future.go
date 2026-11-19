// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/srtp/v3"
)

// srtpWriterFuture blocks Read/Write calls until
// the SRTP Session is available.
//
// RTCP for the encoding arrives on two SRTCP read streams when RTX is
// negotiated: one for the media SSRC and one for the RTX SSRC. A receiver
// puts the SSRC of the last packet it received into its transport-wide
// congestion control feedback, which after a retransmission or RTX padding
// is the RTX SSRC, so feedback addressed to it must reach the sender too.
// Read merges both streams; SetReadDeadline applies to the media stream.
type srtpWriterFuture struct {
	ssrc              SSRC
	rtxSSRC           SSRC
	rtpSender         *RTPSender
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

func (s *srtpWriterFuture) init(returnWhenNoSRTP bool) error { //nolint:cyclop
	// Sending needs SRTP, which otherwise starts only when a negotiation
	// brings media, and never on the ORTC path.
	if err := s.rtpSender.transport.wantSRTP(); err != nil {
		return err
	}
	if returnWhenNoSRTP {
		select {
		case <-s.rtpSender.stopCalled:
			return io.ErrClosedPipe
		case <-s.rtpSender.transport.srtpReady:
		default:
			return nil
		}
	} else {
		select {
		case <-s.rtpSender.stopCalled:
			return io.ErrClosedPipe
		case <-s.rtpSender.transport.srtpReady:
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return io.ErrClosedPipe
	}
	if s.rtcpReadStream.Load() != nil {
		return nil
	}

	srtcpSession, err := s.rtpSender.transport.getSRTCPSession()
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

	srtpSession, err := s.rtpSender.transport.getSRTPSession()
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
func (s *srtpWriterFuture) readRTCPInto(stream *srtp.ReadStreamSRTCP, forwardErrors bool) {
	for {
		buf := make([]byte, s.rtpSender.api.settingEngine.getReceiveMTU())
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

func (s *srtpWriterFuture) Close() error {
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

func (s *srtpWriterFuture) Read(b []byte) (n int, err error) {
	if value := s.rtcpReadStream.Load(); value != nil {
		if s.rtcpRTXReadStream.Load() == nil {
			return value.Read(b)
		}

		return s.readMerged(b)
	}

	if err := s.init(false); err != nil || s.rtcpReadStream.Load() == nil {
		return 0, err
	}

	return s.Read(b)
}

// readMerged returns the next packet from either RTCP read stream.
func (s *srtpWriterFuture) readMerged(b []byte) (int, error) {
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

func (s *srtpWriterFuture) SetReadDeadline(t time.Time) error {
	if value := s.rtcpReadStream.Load(); value != nil {
		return value.SetReadDeadline(t)
	}

	if err := s.init(false); err != nil || s.rtcpReadStream.Load() == nil {
		return err
	}

	return s.SetReadDeadline(t)
}

func (s *srtpWriterFuture) WriteRTP(header *rtp.Header, payload []byte) (int, error) {
	if value := s.rtpWriteStream.Load(); value != nil {
		return value.WriteRTP(header, payload)
	}

	if err := s.init(true); err != nil || s.rtpWriteStream.Load() == nil {
		return 0, err
	}

	return s.WriteRTP(header, payload)
}

func (s *srtpWriterFuture) Write(b []byte) (int, error) {
	if value := s.rtpWriteStream.Load(); value != nil {
		return value.Write(b)
	}

	if err := s.init(true); err != nil || s.rtpWriteStream.Load() == nil {
		return 0, err
	}

	return s.Write(b)
}
