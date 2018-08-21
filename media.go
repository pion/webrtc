package webrtc

import (
	"math/rand"

	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
	"github.com/pions/webrtc/internal/network"
)

// RTCRtpReceiver allows an application to inspect the receipt of a RTCTrack
type RTCRtpReceiver struct {
	Track *RTCTrack
	// receiverTrack *RTCTrack
	// receiverTransport
	// receiverRtcpTransport
}

// TODO: receiving side
// func newRTCRtpReceiver(kind, id string) {
//
// }

// RTCRtpSender allows an application to control how a given RTCTrack is encoded and transmitted to a remote peer
type RTCRtpSender struct {
	Track *RTCTrack
	// senderTrack *RTCTrack
	// senderTransport
	// senderRtcpTransport
}

func newRTCRtpSender(track *RTCTrack) *RTCRtpSender {
	s := &RTCRtpSender{
		Track: track,
	}
	return s
}

// RTCRtpTransceiverDirection indicates the direction of the RTCRtpTransceiver
type RTCRtpTransceiverDirection int

const (

	// RTCRtpTransceiverDirectionSendrecv indicates the RTCRtpSender will offer to send RTP and RTCRtpReceiver the will
	// offer to receive RTP
	RTCRtpTransceiverDirectionSendrecv RTCRtpTransceiverDirection = iota + 1

	// RTCRtpTransceiverDirectionSendonly indicates the RTCRtpSender will offer to send RTP
	RTCRtpTransceiverDirectionSendonly

	// RTCRtpTransceiverDirectionRecvonly indicates the RTCRtpReceiver the will offer to receive RTP
	RTCRtpTransceiverDirectionRecvonly

	// RTCRtpTransceiverDirectionInactive indicates the RTCRtpSender won't offer to send RTP and RTCRtpReceiver the
	// won't offer to receive RTP
	RTCRtpTransceiverDirectionInactive
)

func (t RTCRtpTransceiverDirection) String() string {
	switch t {
	case RTCRtpTransceiverDirectionSendrecv:
		return "sendrecv"
	case RTCRtpTransceiverDirectionSendonly:
		return "sendonly"
	case RTCRtpTransceiverDirectionRecvonly:
		return "recvonly"
	case RTCRtpTransceiverDirectionInactive:
		return "inactive"
	default:
		return ErrUnknownType.Error()
	}
}

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

func (r *RTCPeerConnection) newRTCRtpTransceiver(
	receiver *RTCRtpReceiver,
	sender *RTCRtpSender,
	direction RTCRtpTransceiverDirection,
) *RTCRtpTransceiver {

	t := &RTCRtpTransceiver{
		Receiver:  receiver,
		Sender:    sender,
		Direction: direction,
	}
	r.rtpTransceivers = append(r.rtpTransceivers, t)
	return t
}

// Stop irreversibly stops the RTCRtpTransceiver
func (t *RTCRtpTransceiver) Stop() error {
	return errors.Errorf("TODO")
}

// RTCSample contains media, and the amount of samples in it
type RTCSample struct {
	Data    []byte
	Samples uint32
}

// RTCTrack represents a track that is communicated
type RTCTrack struct {
	ID          string
	PayloadType uint8
	Kind        RTCRtpCodecType
	Label       string
	Ssrc        uint32
	Codec       *RTCRtpCodec

	// You can dequeue RTP packets from this channel
	IncomingPackets <-chan *rtp.Packet

	// If the track is outgoing, you can push RTP packets to this channel and they will be sent on the track
	OutgoingPackets chan *rtp.Packet
}

// NewRTCTrack is used to create a new RTCTrack
func (r *RTCPeerConnection) NewRTCTrack(payloadType uint8, id, label string) (*RTCTrack, error) {
	codec, err := r.mediaEngine.getCodec(payloadType)
	if err != nil {
		return nil, err
	}

	if codec.Payloader == nil {
		return nil, errors.New("codec payloader not set")
	}

	ssrc := rand.Uint32()

	t := &RTCTrack{
		PayloadType:     payloadType,
		Kind:            codec.Type,
		ID:              id,
		Label:           label,
		Ssrc:            ssrc,
		Codec:           codec,
		IncomingPackets: nil,
		OutgoingPackets: make(chan *rtp.Packet),
	}

	go t.sendToTrackPump(r.networkManager)

	return t, nil
}

// SendToTrackPump forwards RTP packets to the track
func (track *RTCTrack) sendToTrackPump(manager *network.Manager) {
	for {
		p := <-track.OutgoingPackets
		p.SSRC = track.Ssrc
		manager.SendRTP(p)
	}
}

// AddTrack adds a RTCTrack to the RTCPeerConnection
func (r *RTCPeerConnection) AddTrack(track *RTCTrack) (*RTCRtpSender, error) {
	if r.IsClosed {
		return nil, &InvalidStateError{Err: ErrConnectionClosed}
	}

	// Make sure the track has not already been added
	for _, transceiver := range r.rtpTransceivers {
		if transceiver.Sender.Track == nil {
			continue
		}
		if track.ID == transceiver.Sender.Track.ID {
			return nil, &InvalidAccessError{Err: ErrExistingTrack}
		}
	}

	// Try to find a transceiver (of the same Kind as the track) that already has a receiver but not sender
	// so we can set the sender.
	var transceiver *RTCRtpTransceiver
	for _, t := range r.rtpTransceivers {
		if !t.stopped &&
		// t.Sender == nil && // TODO: check that the sender has never sent
			t.Sender.Track == nil &&
			t.Receiver.Track != nil &&
			t.Receiver.Track.Kind == track.Kind {
			transceiver = t
			break
		}
	}

	if transceiver != nil {
		if err := transceiver.setSendingTrack(track); err != nil {
			return nil, err
		}
	} else {
		// If we found no matching transceiver, we create a new one that has a sender but no receiver
		transceiver = r.newRTCRtpTransceiver(
			nil,
			newRTCRtpSender(track),
			RTCRtpTransceiverDirectionSendonly,
		)
	}

	transceiver.Mid = track.Kind.String() // TODO: Mid generation

	return transceiver.Sender, nil
}

// GetSenders returns the RTCRtpSender that are currently attached to this RTCPeerConnection
func (r *RTCPeerConnection) GetSenders() []RTCRtpSender {
	result := make([]RTCRtpSender, len(r.rtpTransceivers))
	for i, tranceiver := range r.rtpTransceivers {
		result[i] = *tranceiver.Sender
	}
	return result
}

// GetReceivers returns the RTCRtpReceivers that are currently attached to this RTCPeerConnection
func (r *RTCPeerConnection) GetReceivers() []RTCRtpReceiver {
	result := make([]RTCRtpReceiver, len(r.rtpTransceivers))
	for i, tranceiver := range r.rtpTransceivers {
		result[i] = *tranceiver.Receiver
	}
	return result
}

// GetTransceivers returns the RTCRtpTransceiver that are currently attached to this RTCPeerConnection
func (r *RTCPeerConnection) GetTransceivers() []RTCRtpTransceiver {
	result := make([]RTCRtpTransceiver, len(r.rtpTransceivers))
	for i, tranceiver := range r.rtpTransceivers {
		result[i] = *tranceiver
	}
	return result
}
