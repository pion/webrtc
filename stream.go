// +build !js

package webrtc

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v2/pkg/media"
)

const (
	rtpOutboundMTU = 1200
)

// TrackRTPStream represents a single rtp stream
type TrackRTPStream struct {
	mu sync.RWMutex

	// stream id, if rid based it's the rid, it ssrc based it's the ssrc as string
	id string

	ready bool

	payloadType uint8
	rid         string
	ssrc        uint32
	codec       *RTPCodec

	packetizer rtp.Packetizer

	track *Track
}

// Ready reports if the stream is ready. If it's a read stream it'll
// be ready when it's SSRC, RID if rid based, payload are know. If
// it's a write stream it'll always be ready
func (s *TrackRTPStream) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// RID gets the RID of the stream. If this isn't a rid based stream
// it'll return an empty value
func (s *TrackRTPStream) RID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rid
}

// PayloadType gets the PayloadType of the stream. If this is a rid
// based stream the payload may be not yet known until a packet is
// received
func (s *TrackRTPStream) PayloadType() uint8 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.payloadType
}

// SSRC gets the current SSRC of the stream. If a stream is RID based,
// this is the currently known ssrc (may be 0 until a packet is
// received) for the stream rid and may change
func (s *TrackRTPStream) SSRC() uint32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ssrc
}

// Codec gets the Codec of the stream. If this is a rid based stream
// the codec may be nil until a packet is received
func (s *TrackRTPStream) Codec() *RTPCodec {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.codec
}

// Packetizer gets the Packetizer of the stream
func (s *TrackRTPStream) Packetizer() rtp.Packetizer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.packetizer
}

// Read reads data from the stream. If this is a local stream this will error
func (s *TrackRTPStream) Read(b []byte) (n int, err error) {
	return s.track.read(b, s.rid)
}

// ReadRTP is a convenience method that wraps Read and unmarshals for you
func (s *TrackRTPStream) ReadRTP() (*rtp.Packet, error) {
	b := make([]byte, receiveMTU)
	i, err := s.Read(b)
	if err != nil {
		return nil, err
	}

	r := &rtp.Packet{}
	if err := r.Unmarshal(b[:i]); err != nil {
		return nil, err
	}
	return r, nil
}

// Write writes data to the stream. If this is a remote stream this will error
func (s *TrackRTPStream) Write(b []byte) (n int, err error) {
	packet := &rtp.Packet{}
	err = packet.Unmarshal(b)
	if err != nil {
		return 0, err
	}

	err = s.WriteRTP(packet)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

// WriteSample packetizes and writes to the stream
func (s *TrackRTPStream) WriteSample(sample media.Sample) error {
	packets := s.packetizer.Packetize(sample.Data, sample.Samples)
	for _, p := range packets {
		err := s.WriteRTP(p)
		if err != nil {
			return err
		}
	}

	return nil
}

// WriteRTP writes RTP packets to the stream
func (s *TrackRTPStream) WriteRTP(p *rtp.Packet) error {
	return s.track.WriteRTP(p)
}

// NewTrackRTPStream initializes a new RTPStream
func NewTrackRTPStream(rid string, payloadType uint8, ssrc uint32, codec *RTPCodec) (*TrackRTPStream, error) {
	if ssrc == 0 {
		return nil, fmt.Errorf("SSRC supplied to NewStream() must be non-zero")
	}

	streamID := strconv.FormatUint(uint64(ssrc), 10)
	// if rid is not empty use it as stream id
	if rid != "" {
		streamID = rid
	}

	packetizer := rtp.NewPacketizer(
		rtpOutboundMTU,
		payloadType,
		ssrc,
		codec.Payloader,
		rtp.NewRandomSequencer(),
		codec.ClockRate,
	)

	return &TrackRTPStream{
		id:          streamID,
		rid:         rid,
		payloadType: payloadType,
		ssrc:        ssrc,
		codec:       codec,
		packetizer:  packetizer,
	}, nil
}

// determinePayloadType blocks and reads a single packet to determine the PayloadType for this Stream
// This is useful if we are dealing with a remote stream and we can't announce it to the user until we know the payloadType
func (s *TrackRTPStream) determinePayloadType() error {
	r, err := s.ReadRTP()
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.payloadType = r.PayloadType
	s.mu.Unlock()

	return nil
}
