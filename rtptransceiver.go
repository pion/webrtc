package webrtc

import (
	"github.com/pkg/errors"
)

// RTPTransceiver represents a combination of an RTPSender and an RTPReceiver that share a common mid.
type RTPTransceiver struct {
	Mid       string
	Sender    *RTPSender
	Receiver  *RTPReceiver
	Direction RTPTransceiverDirection
	// currentDirection RTPTransceiverDirection
	// firedDirection   RTPTransceiverDirection
	// receptive bool
	stopped bool
}

func (t *RTPTransceiver) setSendingTrack(track *Track) error {
	t.Sender.track = track

	switch t.Direction {
	case RTPTransceiverDirectionRecvonly:
		t.Direction = RTPTransceiverDirectionSendrecv
	case RTPTransceiverDirectionInactive:
		t.Direction = RTPTransceiverDirectionSendonly
	default:
		return errors.Errorf("Invalid state change in RTPTransceiver.setSending")
	}
	return nil
}

// Stop irreversibly stops the RTPTransceiver
func (t *RTPTransceiver) Stop() error {
	if t.Sender != nil {
		if err := t.Sender.Stop(); err != nil {
			return err
		}
	}
	if t.Receiver != nil {
		if err := t.Receiver.Stop(); err != nil {
			return err
		}
	}
	return nil
}
