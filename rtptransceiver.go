// +build !js

package webrtc

import (
	"fmt"
	"sync/atomic"
)

// RTPTransceiver represents a combination of an RTPSender and an RTPReceiver that share a common mid.
type RTPTransceiver struct {
	sender    atomic.Value // *RTPSender
	receiver  atomic.Value // *RTPReceiver
	direction atomic.Value // RTPTransceiverDirection

	stopped bool
	kind    RTPCodecType
}

// Sender returns the RTPTransceiver's RTPSender if it has one
func (t *RTPTransceiver) Sender() *RTPSender {
	if v := t.sender.Load(); v != nil {
		return v.(*RTPSender)
	}

	return nil
}

func (t *RTPTransceiver) setSender(s *RTPSender) {
	t.sender.Store(s)
}

// Receiver returns the RTPTransceiver's RTPReceiver if it has one
func (t *RTPTransceiver) Receiver() *RTPReceiver {
	if v := t.receiver.Load(); v != nil {
		return v.(*RTPReceiver)
	}

	return nil
}

// Direction returns the RTPTransceiver's current direction
func (t *RTPTransceiver) Direction() RTPTransceiverDirection {
	return t.direction.Load().(RTPTransceiverDirection)
}

// Stop irreversibly stops the RTPTransceiver
func (t *RTPTransceiver) Stop() error {
	if t.Sender() != nil {
		if err := t.Sender().Stop(); err != nil {
			return err
		}
	}
	if t.Receiver() != nil {
		if err := t.Receiver().Stop(); err != nil {
			return err
		}
	}

	t.setDirection(RTPTransceiverDirectionInactive)
	return nil
}

func (t *RTPTransceiver) setReceiver(r *RTPReceiver) {
	t.receiver.Store(r)
}

func (t *RTPTransceiver) setDirection(d RTPTransceiverDirection) {
	t.direction.Store(d)
}

func (t *RTPTransceiver) setSendingTrack(track *Track) error {
	t.Sender().track = track
	if track == nil {
		t.setSender(nil)
	}

	switch {
	case track != nil && t.Direction() == RTPTransceiverDirectionRecvonly:
		t.setDirection(RTPTransceiverDirectionSendrecv)
	case track != nil && t.Direction() == RTPTransceiverDirectionInactive:
		t.setDirection(RTPTransceiverDirectionSendonly)
	case track == nil && t.Direction() == RTPTransceiverDirectionSendrecv:
		t.setDirection(RTPTransceiverDirectionRecvonly)
	case track == nil && t.Direction() == RTPTransceiverDirectionSendonly:
		t.setDirection(RTPTransceiverDirectionInactive)
	default:
		return fmt.Errorf("invalid state change in RTPTransceiver.setSending")
	}
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
			if t.kind != remoteKind || possibleDirection != t.Direction() {
				continue
			}

			return t, append(localTransceivers[:i], localTransceivers[i+1:]...)
		}
	}

	d := atomic.Value{}
	d.Store(RTPTransceiverDirectionInactive)

	return &RTPTransceiver{
		kind:      remoteKind,
		direction: d,
	}, localTransceivers
}
