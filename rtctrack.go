package webrtc

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/pions/rtcp"
	"github.com/pions/rtp"
	"github.com/pions/webrtc/pkg/media"
	"github.com/pkg/errors"
)

// Track represents a track that is communicated
type Track struct {
	isRawRTP    bool
	sampleInput chan media.Sample
	rawInput    chan *rtp.Packet
	rtcpInput   chan rtcp.Packet

	ID          string
	PayloadType uint8
	Kind        RTPCodecType
	Label       string
	Ssrc        uint32
	Codec       *RTPCodec

	Packets     <-chan *rtp.Packet
	RTCPPackets <-chan rtcp.Packet

	Samples chan<- media.Sample
	RawRTP  chan<- *rtp.Packet
}

// NewRawRTPTrack initializes a new *Track configured to accept raw *rtp.Packet
//
// NB: If the source RTP stream is being broadcast to multiple tracks, each track
// must receive its own copies of the source packets in order to avoid packet corruption.
func NewRawRTPTrack(payloadType uint8, ssrc uint32, id, label string, codec *RTPCodec) (*Track, error) {
	if ssrc == 0 {
		return nil, errors.New("SSRC supplied to NewRawRTPTrack() must be non-zero")
	}

	return &Track{
		isRawRTP: true,

		ID:          id,
		PayloadType: payloadType,
		Kind:        codec.Type,
		Label:       label,
		Ssrc:        ssrc,
		Codec:       codec,
	}, nil
}

// NewSampleTrack initializes a new *Track configured to accept media.Sample
func NewSampleTrack(payloadType uint8, id, label string, codec *RTPCodec) (*Track, error) {
	if codec == nil {
		return nil, errors.New("codec supplied to NewSampleTrack() must not be nil")
	}

	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return nil, errors.New("failed to generate random value")
	}

	return &Track{
		isRawRTP: false,

		ID:          id,
		PayloadType: payloadType,
		Kind:        codec.Type,
		Label:       label,
		Ssrc:        binary.LittleEndian.Uint32(buf),
		Codec:       codec,
	}, nil
}
