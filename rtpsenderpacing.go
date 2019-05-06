package webrtc

import (
	"errors"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

type (
	// RTCPSource defines an interface to be implemented
	// by RTCP sources
	RTCPSource interface {
		ReadRTCP([]byte) (int, *rtcp.Header, error)
	}

	// RTPSink defines an interface to be implemented
	// by RTP writers
	RTPSink interface {
		WriteRTP(*rtp.Header, []byte) (int, error)
	}

	// RTPSenderPacingStrategy defines an interface
	// to be implemented by RTPSender pacing strategies
	RTPSenderPacingStrategy interface {
		SendRTP(*rtp.Header, []byte) (int, error)
		SetRTCPSource(RTCPSource) error
		SetRTPSink(RTPSink) error
		Stop() error
	}

	// RTPSenderPacingPassthrough is the default pacing strategy
	// which just passes RTP through to the underlying RTPSink
	// and does nothing with RTCP
	RTPSenderPacingPassthrough struct {
		rtpSink RTPSink
	}
)

// SetRTCPSource implements the RTPSenderPacingStrategy interface,
// however this strategy implementation does not support modification
// after initialization
func (rsp *RTPSenderPacingPassthrough) SetRTCPSource(s RTCPSource) error {
	return errors.New("RTPSenderPacingPassthrough does not support modification of RTCPSource")
}

// SetRTPSink implements the RTPSenderPacingStrategy interface,
// however this strategy implementation does not support modification
// after initialization
func (rsp *RTPSenderPacingPassthrough) SetRTPSink(rtpSink RTPSink) error {
	return errors.New("RTPSenderPacingPassthrough does not support modification of RTPSink")
}

// Stop implements theRTPSenderPacingStrategy interface, however
// this strategy implementation does not do anything with it
func (rsp *RTPSenderPacingPassthrough) Stop() error {
	// noop
	return nil
}

// SendRTP writes the RTP directly to the RTPSink configured when this strategy
// was initialized
func (rsp *RTPSenderPacingPassthrough) SendRTP(header *rtp.Header, payload []byte) (int, error) {
	// We don't need to lock because the rtpSink is set when the struct is created
	return rsp.rtpSink.WriteRTP(header, payload)
}
