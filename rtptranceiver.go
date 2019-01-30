package webrtc

import (
	"fmt"

	"github.com/pkg/errors"
)

// RTPTransceiver represents a combination of an RTCRtpSender and an RTCRtpReceiver that share a common mid.
type RTPTransceiver struct {
	Mid       string
	Sender    *RTPSender
	Receiver  *RTPReceiver
	Direction RTPTransceiverDirection
	// currentDirection RTPTransceiverDirection
	// firedDirection   RTPTransceiverDirection
	// receptive bool
	stopped bool

	remoteCapabilities     RTPParameters
	recvDecodingParameters []RTPDecodingParameters

	peerConnection *PeerConnection
}

func (t *RTPTransceiver) setSendingTrack(track *Track) error {
	t.Sender.Track = track

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

func (t *RTPTransceiver) start() error {
	// Start the sender
	sender := t.Sender
	if sender != nil {
		sender.Send(RTPSendParameters{
			encodings: RTPEncodingParameters{
				RTPCodingParameters{SSRC: sender.Track.SSRC, PayloadType: sender.Track.PayloadType},
			}})
	}

	// Start the receiver
	receiver := t.Receiver
	if receiver != nil {
		params := RTPReceiveParameters{
			RTPParameters: t.remoteCapabilities,
			Encodings:     t.recvDecodingParameters,
		}

		err := receiver.Receive(params)
		if err != nil {
			return fmt.Errorf("failed to receive: %v", err)
		}

		t.peerConnection.onTrack(receiver.Track)
	}

	return nil
}

// Stop irreversibly stops the RTCRtpTransceiver
func (t *RTPTransceiver) Stop() error {
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
