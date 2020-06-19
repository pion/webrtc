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
	rtpOutboundMTU          = 1200
	trackDefaultIDLength    = 16
	trackDefaultLabelLength = 16
)

// Track represents a single media track
type Track struct {
	mu sync.RWMutex

	id          string
	payloadType uint8
	kind        RTPCodecType
	label       string
	ssrc        uint32
	codec       *RTPCodec

	packetizer rtp.Packetizer

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
	return t.payloadType
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

// SSRC gets the SSRC of the track
func (t *Track) SSRC() uint32 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ssrc
}

// Codec gets the Codec of the track
func (t *Track) Codec() *RTPCodec {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.codec
}

// Packetizer gets the Packetizer of the track
func (t *Track) Packetizer() rtp.Packetizer {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.packetizer
}

// Read reads data from the track. If this is a local track this will error
func (t *Track) Read(b []byte) (n int, err error) {
	t.mu.RLock()
	r := t.receiver

	if t.totalSenderCount != 0 || r == nil {
		t.mu.RUnlock()
		return 0, fmt.Errorf("this is a local track and must not be read from")
	}
	t.mu.RUnlock()

	return r.readRTP(b)
}

// ReadRTP is a convenience method that wraps Read and unmarshals for you
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

// Write writes data to the track. If this is a remote track this will error
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

// WriteSample packetizes and writes to the track
func (t *Track) WriteSample(s media.Sample) error {
	packets := t.packetizer.Packetize(s.Data, s.Samples)
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

// NewTrack initializes a new *Track
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

	return &Track{
		id:          id,
		payloadType: payloadType,
		kind:        codec.Type,
		label:       label,
		ssrc:        ssrc,
		codec:       codec,
		packetizer:  packetizer,
	}, nil
}

// determinePayloadType blocks and reads a single packet to determine the PayloadType for this Track
// this is useful if we are dealing with a remote track and we can't announce it to the user until we know the payloadType
func (t *Track) determinePayloadType() error {
	r, err := t.ReadRTP()
	if err != nil {
		return err
	}

	t.mu.Lock()
	t.payloadType = r.PayloadType
	defer t.mu.Unlock()

	return nil
}
