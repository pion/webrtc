package webrtc

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/pions/rtcp"
	"github.com/pions/rtp"
	"github.com/pions/webrtc/pkg/media"
	"github.com/pkg/errors"
)

// RTCTrack represents a track that is communicated
type RTCTrack struct {
	isRawRTP bool

	ID          string
	PayloadType uint8
	Kind        RTCRtpCodecType
	Label       string
	Ssrc        uint32
	Codec       *RTCRtpCodec

	Packets     <-chan *rtp.Packet
	RTCPPackets <-chan rtcp.Packet

	Samples chan<- media.RTCSample
	RawRTP  chan<- *rtp.Packet
}

// NewRawRTPTrack initializes a new *RTCTrack configured to accept raw *rtp.Packet
//
// NB: If the source RTP stream is being broadcast to multiple tracks, each track
// must receive its own copies of the source packets in order to avoid packet corruption.
func NewRawRTPTrack(payloadType uint8, ssrc uint32, id, label string, codec *RTCRtpCodec) (*RTCTrack, error) {
	if ssrc == 0 {
		return nil, errors.New("SSRC supplied to NewRawRTPTrack() must be non-zero")
	}

	return &RTCTrack{
		isRawRTP: true,

		ID:          id,
		PayloadType: payloadType,
		Kind:        codec.Type,
		Label:       label,
		Ssrc:        ssrc,
		Codec:       codec,
	}, nil
}

// NewRTCSampleTrack initializes a new *RTCTrack configured to accept media.RTCSample
func NewRTCSampleTrack(payloadType uint8, id, label string, codec *RTCRtpCodec) (*RTCTrack, error) {
	if codec == nil {
		return nil, errors.New("codec supplied to NewRTCSampleTrack() must not be nil")
	}

	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return nil, errors.New("failed to generate random value")
	}

	return &RTCTrack{
		isRawRTP: false,

		ID:          id,
		PayloadType: payloadType,
		Kind:        codec.Type,
		Label:       label,
		Ssrc:        binary.LittleEndian.Uint32(buf),
		Codec:       codec,
	}, nil
}
