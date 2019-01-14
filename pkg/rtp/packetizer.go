package rtp

import (
	"math/rand"
	"time"
)

// Payloader payloads a byte array for use as rtp.Packet payloads
type Payloader interface {
	Payload(mtu int, payload []byte) [][]byte
}

// Packetizer packetizes a payload
type Packetizer interface {
	Packetize(payload []byte, samples uint32) []*Packet
}

type packetizer struct {
	MTU         int
	PayloadType uint8
	SSRC        uint32
	Payloader   Payloader
	Sequencer   Sequencer
	Timestamp   uint32
	ClockRate   uint32
}

// NewPacketizer returns a new instance of a Packetizer for a specific payloader
func NewPacketizer(mtu int, pt uint8, ssrc uint32, payloader Payloader, sequencer Sequencer, clockRate uint32) Packetizer {
	rs := rand.NewSource(time.Now().UnixNano())
	r := rand.New(rs)

	return &packetizer{
		MTU:         mtu,
		PayloadType: pt,
		SSRC:        ssrc,
		Payloader:   payloader,
		Sequencer:   sequencer,
		Timestamp:   r.Uint32(),
		ClockRate:   clockRate,
	}
}

// Packetize packetizes the payload of an RTP packet and returns one or more RTP packets
func (p *packetizer) Packetize(payload []byte, samples uint32) []*Packet {
	// Guard against an empty payload
	if len(payload) == 0 {
		return nil
	}

	payloads := p.Payloader.Payload(p.MTU-12, payload)
	packets := make([]*Packet, len(payloads))

	for i, pp := range payloads {
		packets[i] = &Packet{
			Header: Header{
				Version:        2,
				Padding:        false,
				Extension:      false,
				Marker:         i == len(payloads)-1,
				PayloadType:    p.PayloadType,
				SequenceNumber: p.Sequencer.NextSequenceNumber(),
				Timestamp:      p.Timestamp, // Figure out how to do timestamps
				SSRC:           p.SSRC,
			},
			Payload: pp,
		}
	}
	p.Timestamp += samples

	return packets
}
