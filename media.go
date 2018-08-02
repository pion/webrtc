package webrtc

import (
	"math/rand"

	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
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

	// RTCRtpTransceiverDirectionSendrecv indicates the RTCRtpSender will offer to send RTP and RTCRtpReceiver the will offer to receive RTP
	RTCRtpTransceiverDirectionSendrecv RTCRtpTransceiverDirection = iota + 1

	// RTCRtpTransceiverDirectionSendonly indicates the RTCRtpSender will offer to send RTP
	RTCRtpTransceiverDirectionSendonly

	// RTCRtpTransceiverDirectionRecvonly indicates the RTCRtpReceiver the will offer to receive RTP
	RTCRtpTransceiverDirectionRecvonly

	// RTCRtpTransceiverDirectionInactive indicates the RTCRtpSender won't offer to send RTP and RTCRtpReceiver the won't offer to receive RTP
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
		return "Unknown"
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

func (t *RTCRtpTransceiver) setSendingTrack(track *RTCTrack) {
	t.Sender.Track = track

	switch t.Direction {
	case RTCRtpTransceiverDirectionRecvonly:
		t.Direction = RTCRtpTransceiverDirectionSendrecv
	case RTCRtpTransceiverDirectionInactive:
		t.Direction = RTCRtpTransceiverDirectionSendonly
	default:
		panic("Invalid state change in RTCRtpTransceiver.setSending")
	}
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
	panic("TODO")
}

// RTCSample contains media, and the amount of samples in it
type RTCSample struct {
	Data    []byte
	Samples uint32
}

// RTCTrack represents a track that is communicated
type RTCTrack struct {
	PayloadType uint8
	Kind        RTCRtpCodecType
	ID          string
	Label       string
	Ssrc        uint32
	Codec       *RTCRtpCodec
	Packets     <-chan *rtp.Packet
	Samples     chan<- RTCSample
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

	trackInput := make(chan RTCSample, 15) // Is the buffering needed?
	ssrc := rand.Uint32()
	go func() {
		packetizer := rtp.NewPacketizer(
			1400,
			payloadType,
			ssrc,
			codec.Payloader,
			rtp.NewRandomSequencer(),
			codec.ClockRate,
		)
		for {
			in := <-trackInput
			packets := packetizer.Packetize(in.Data, in.Samples)
			for _, p := range packets {
				r.networkManager.SendRTP(p)
			}
		}
	}()

	t := &RTCTrack{
		PayloadType: payloadType,
		Kind:        codec.Type,
		ID:          id,
		Label:       label,
		Ssrc:        ssrc,
		Codec:       codec,
		Samples:     trackInput,
	}

	return t, nil
}

// AddTrack adds a RTCTrack to the RTCPeerConnection
func (r *RTCPeerConnection) AddTrack(track *RTCTrack) (*RTCRtpSender, error) {
	if r.IsClosed {
		return nil, &InvalidStateError{Err: ErrConnectionClosed}
	}
	for _, transceiver := range r.rtpTransceivers {
		if transceiver.Sender.Track == nil {
			continue
		}
		if track.ID == transceiver.Sender.Track.ID {
			return nil, &InvalidAccessError{Err: ErrExistingTrack}
		}
	}
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
		transceiver.setSendingTrack(track)
	} else {
		var receiver *RTCRtpReceiver
		sender := newRTCRtpSender(track)
		transceiver = r.newRTCRtpTransceiver(
			receiver,
			sender,
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
