// +build !js

package webrtc

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/pion/rtp"
	"github.com/pion/srtp/v2"
)

// srtpWriterFuture blocks Read/Write calls until
// the SRTP Session is available
type srtpWriterFuture struct {
	rtpSender      *RTPSender
	rtcpReadStream atomic.Value // *srtp.ReadStreamSRTCP
	rtpWriteStream atomic.Value // *srtp.WriteStreamSRTP
}

func (s *srtpWriterFuture) init(ctx context.Context) error {
	select {
	case <-s.rtpSender.stopCalled:
		return io.ErrClosedPipe
	case <-s.rtpSender.transport.srtpReady:
	case <-ctx.Done():
		return ctx.Err()
	}

	srtcpSession, err := s.rtpSender.transport.getSRTCPSession()
	if err != nil {
		return err
	}

	rtcpReadStream, err := srtcpSession.OpenReadStream(uint32(s.rtpSender.ssrc))
	if err != nil {
		return err
	}

	srtpSession, err := s.rtpSender.transport.getSRTPSession()
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

func (s *srtpWriterFuture) Close() error {
	if value := s.rtcpReadStream.Load(); value != nil {
		return value.(*srtp.ReadStreamSRTCP).Close()
	}

	return nil
}

func (s *srtpWriterFuture) ReadContext(ctx context.Context, b []byte) (n int, err error) {
	if value := s.rtcpReadStream.Load(); value != nil {
		return value.(*srtp.ReadStreamSRTCP).ReadContext(ctx, b)
	}

	if err := s.init(ctx); err != nil || s.rtcpReadStream.Load() == nil {
		return 0, err
	}

	return s.ReadContext(ctx, b)
}

func (s *srtpWriterFuture) WriteRTP(ctx context.Context, header *rtp.Header, payload []byte) (int, error) {
	if value := s.rtpWriteStream.Load(); value != nil {
		return value.(*srtp.WriteStreamSRTP).WriteRTP(ctx, header, payload)
	}

	if err := s.init(ctx); err != nil || s.rtpWriteStream.Load() == nil {
		return 0, err
	}

	return s.WriteRTP(ctx, header, payload)
}

func (s *srtpWriterFuture) Write(ctx context.Context, b []byte) (int, error) {
	if value := s.rtpWriteStream.Load(); value != nil {
		return value.(*srtp.WriteStreamSRTP).WriteContext(ctx, b)
	}

	if err := s.init(ctx); err != nil || s.rtpWriteStream.Load() == nil {
		return 0, err
	}

	return s.Write(ctx, b)
}
