package webrtc

import (
	"github.com/pkg/errors"
)

// RTCRtpTransceiver represents a combination of an RTCRtpSender and an RTCRtpReceiver that share a common mid.
type RTCRtpTransceiver struct {
	Mid       string
	Sender    *RTCRtpSender
	Receiver  *RTCRtpReceiver
	Direction RTCRtpTransceiverDirection
	// currentDirection RTCRtpTransceiverDirection
	// firedDirection   RTCRtpTransceiverDirection
	// receptive bool
	stopped bool
}

func (t *RTCRtpTransceiver) setSendingTrack(track *RTCTrack) error {
	t.Sender.Track = track

	switch t.Direction {
	case RTCRtpTransceiverDirectionRecvonly:
		t.Direction = RTCRtpTransceiverDirectionSendrecv
	case RTCRtpTransceiverDirectionInactive:
		t.Direction = RTCRtpTransceiverDirectionSendonly
	default:
		return errors.Errorf("Invalid state change in RTCRtpTransceiver.setSending")
	}
	return nil
}

// Stop irreversibly stops the RTCRtpTransceiver
func (t *RTCRtpTransceiver) Stop() error {
	if t.Sender != nil {
		t.Sender.Stop()
	}
	if t.Receiver != nil {
		if err := t.Receiver.Stop(); err != nil {
			return err
		}
	}
	return nil
}
