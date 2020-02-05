// +build !js

package webrtc

import (
	"fmt"
)

// RTPTransceiver represents a combination of an RTPSender and an RTPReceiver that share a common mid.
type RTPTransceiver struct {
	Sender    *RTPSender
	Receiver  *RTPReceiver
	Direction RTPTransceiverDirection
	// currentDirection RTPTransceiverDirection
	// firedDirection   RTPTransceiverDirection
	// receptive bool
	stopped bool
	kind    RTPCodecType
}

func (t *RTPTransceiver) setSendingTrack(track *Track) error {
	if track == nil {
		return fmt.Errorf("track must not be nil")
	}

	t.Sender.track = track

	switch t.Direction {
	case RTPTransceiverDirectionRecvonly:
		t.Direction = RTPTransceiverDirectionSendrecv
	case RTPTransceiverDirectionInactive:
		t.Direction = RTPTransceiverDirectionSendonly
	default:
		return fmt.Errorf("invalid state change in RTPTransceiver.setSending")
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

	t.Direction = RTPTransceiverDirectionInactive
	return nil
}

// Given a direction+type pluck a transceiver from the passed list
// if no entry satisfies the requested type+direction return a inactive Transceiver
func satisfyTypeAndDirection(remoteKind RTPCodecType, remoteDirection RTPTransceiverDirection, localTransceivers []*RTPTransceiver) (*RTPTransceiver, []*RTPTransceiver) {
	// Get direction order from most preferred to least
	getPreferredDirections := func() []RTPTransceiverDirection {
		switch remoteDirection {
		case RTPTransceiverDirectionSendrecv:
			return []RTPTransceiverDirection{RTPTransceiverDirectionRecvonly, RTPTransceiverDirectionSendrecv}
		case RTPTransceiverDirectionSendonly:
			return []RTPTransceiverDirection{RTPTransceiverDirectionRecvonly, RTPTransceiverDirectionSendrecv}
		case RTPTransceiverDirectionRecvonly:
			return []RTPTransceiverDirection{RTPTransceiverDirectionSendonly, RTPTransceiverDirectionSendrecv}
		}
		return []RTPTransceiverDirection{}
	}

	for _, possibleDirection := range getPreferredDirections() {
		for i := range localTransceivers {
			t := localTransceivers[i]
			if t.kind != remoteKind || possibleDirection != t.Direction {
				continue
			}

			return t, append(localTransceivers[:i], localTransceivers[i+1:]...)
		}
	}

	return &RTPTransceiver{
		kind:      remoteKind,
		Direction: RTPTransceiverDirectionInactive,
	}, localTransceivers
}
