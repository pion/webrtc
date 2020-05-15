// +build !js

package webrtc

import (
	"fmt"
	"io"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v2/pkg/media"
)

const (
	trackDefaultIDLength    = 16
	trackDefaultLabelLength = 16
)

// Track represents a single media track
type Track struct {
	mu sync.RWMutex

	id    string
	kind  RTPCodecType
	label string

	streams []*TrackRTPStream

	multiStream bool

	receiver         *RTPReceiver
	activeSenders    []*RTPSender
	totalSenderCount int // count of all senders (accounts for senders that have not been started yet)
}

// ID gets the ID of the track
func (t *Track) ID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.id
}

// PayloadType gets the PayloadType of the track
func (t *Track) PayloadType() uint8 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.multiStream {
		return 0
	}
	return t.streams[0].PayloadType()
}

// Kind gets the Kind of the track
func (t *Track) Kind() RTPCodecType {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.kind
}

// Label gets the Label of the track
func (t *Track) Label() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.label
}

// Streams return the track streams
func (t *Track) Streams() []*TrackRTPStream {
	t.mu.RLock()
	defer t.mu.RUnlock()

	streams := make([]*TrackRTPStream, len(t.streams))
	copy(streams, t.streams)
	return streams
}

// SSRC gets the SSRC of the track. If a track is multistream it'll
// return a 0 value (use TrackStream.SSRC())
func (t *Track) SSRC() uint32 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.multiStream {
		return 0
	}
	return t.streams[0].SSRC()
}

// Codec gets the Codec of the track. If a track is multistream it'll
// return a nil value (use TrackStream.Codec())
func (t *Track) Codec() *RTPCodec {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.multiStream {
		return nil
	}
	return t.streams[0].Codec()
}

// Packetizer gets the Packetizer of the track. If a track is
// multistream it'll return a nil value (use TrackStream.Packetizer())
func (t *Track) Packetizer() rtp.Packetizer {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.multiStream {
		return nil
	}
	return t.streams[0].Packetizer()
}

// Write writes data to the track. If this is a remote track this will
// error
func (t *Track) Write(b []byte) (n int, err error) {
	packet := &rtp.Packet{}
	err = packet.Unmarshal(b)
	if err != nil {
		return 0, err
	}

	err = t.WriteRTP(packet)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

// WriteSample packetizes and writes to the track. If a track is
// multistream it'll return a nil value (use TrackStream.WriteSample())
func (t *Track) WriteSample(s media.Sample) error {
	if t.multiStream {
		return fmt.Errorf("track is multistream")
	}
	packets := t.streams[0].packetizer.Packetize(s.Data, s.Samples)
	for _, p := range packets {
		err := t.WriteRTP(p)
		if err != nil {
			return err
		}
	}

	return nil
}

// WriteRTP writes RTP packets to the track
func (t *Track) WriteRTP(p *rtp.Packet) error {
	t.mu.RLock()
	if t.receiver != nil {
		t.mu.RUnlock()
		return fmt.Errorf("this is a remote track and must not be written to")
	}
	senders := t.activeSenders
	totalSenderCount := t.totalSenderCount
	t.mu.RUnlock()

	if totalSenderCount == 0 {
		return io.ErrClosedPipe
	}

	for _, s := range senders {
		_, err := s.SendRTP(&p.Header, p.Payload)
		if err != nil {
			return err
		}
	}

	return nil
}

// NewTrack initializes a new *Track. Currently only single stream tracks can be created
func NewTrack(payloadType uint8, ssrc uint32, id, label string, codec *RTPCodec) (*Track, error) {
	if ssrc == 0 {
		return nil, fmt.Errorf("SSRC supplied to NewTrack() must be non-zero")
	}

	packetizer := rtp.NewPacketizer(
		rtpOutboundMTU,
		payloadType,
		ssrc,
		codec.Payloader,
		rtp.NewRandomSequencer(),
		codec.ClockRate,
	)

	stream := &TrackRTPStream{
		payloadType: payloadType,
		ssrc:        ssrc,
		codec:       codec,
		packetizer:  packetizer,
	}

	return &Track{
		id:      id,
		kind:    codec.Type,
		label:   label,
		streams: []*TrackRTPStream{stream},
	}, nil
}

func (t *Track) read(b []byte, streamID string) (n int, err error) {
	t.mu.RLock()
	if len(t.activeSenders) != 0 {
		t.mu.RUnlock()
		return 0, fmt.Errorf("this is a local track and must not be read from")
	}
	r := t.receiver
	t.mu.RUnlock()

	return r.readRTP(b)
}

// Read reads data from the track. If this is a local track this will
// error. If a track is multistream it'll return an error (use TrackStream.Read())
func (t *Track) Read(b []byte) (n int, err error) {
	if t.multiStream {
		return 0, fmt.Errorf("track is multistream")
	}
	return t.read(b, t.streams[0].id)
}

// ReadRTP is a convenience method that wraps Read and unmarshals for
// you. If a track is multistream it'll return an error (use TrackStream.ReadRTP())
func (t *Track) ReadRTP() (*rtp.Packet, error) {
	b := make([]byte, receiveMTU)
	i, err := t.Read(b)
	if err != nil {
		return nil, err
	}

	r := &rtp.Packet{}
	if err := r.Unmarshal(b[:i]); err != nil {
		return nil, err
	}
	return r, nil
}
