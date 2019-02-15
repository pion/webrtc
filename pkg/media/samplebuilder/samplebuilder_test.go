package samplebuilder

import (
	"testing"

	"github.com/pions/rtp"
	"github.com/pions/webrtc/pkg/media"
	"github.com/stretchr/testify/assert"
)

type sampleBuilderTest struct {
	message string
	packets []*rtp.Packet
	samples []*media.Sample
	maxLate uint16
}

type fakeDepacketizer struct {
}

func (f *fakeDepacketizer) Unmarshal(packet *rtp.Packet) ([]byte, error) {
	return packet.Payload, nil
}

var testCases = []sampleBuilderTest{
	{
		message: "SampleBuilder shouldn't emit anything if only one RTP packet has been pushed",
		packets: []*rtp.Packet{
			{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
		},
		samples: []*media.Sample{},
		maxLate: 50,
	},
	{
		message: "SampleBuilder should emit one packet, we had three packets with unique timestamps",
		packets: []*rtp.Packet{
			{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
			{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 6}, Payload: []byte{0x02}},
			{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 7}, Payload: []byte{0x03}},
		},
		samples: []*media.Sample{
			{Data: []byte{0x02}, Samples: 1},
		},
		maxLate: 50,
	},
	{
		message: "SampleBuilder should emit one packet, we had two packets but two with duplicate timestamps",
		packets: []*rtp.Packet{
			{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
			{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 6}, Payload: []byte{0x02}},
			{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 6}, Payload: []byte{0x03}},
			{Header: rtp.Header{SequenceNumber: 5003, Timestamp: 7}, Payload: []byte{0x04}},
		},
		samples: []*media.Sample{
			{Data: []byte{0x02, 0x03}, Samples: 1},
		},
		maxLate: 50,
	},
	{
		message: "SampleBuilder shouldn't emit a packet because we have a gap before a valid one",
		packets: []*rtp.Packet{
			{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
			{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
			{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
		},
		samples: []*media.Sample{},
		maxLate: 50,
	},
	{
		message: "SampleBuilder should emit multiple valid packets",
		packets: []*rtp.Packet{
			{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 1}, Payload: []byte{0x01}},
			{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 2}, Payload: []byte{0x02}},
			{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 3}, Payload: []byte{0x03}},
			{Header: rtp.Header{SequenceNumber: 5003, Timestamp: 4}, Payload: []byte{0x04}},
			{Header: rtp.Header{SequenceNumber: 5004, Timestamp: 5}, Payload: []byte{0x05}},
			{Header: rtp.Header{SequenceNumber: 5005, Timestamp: 6}, Payload: []byte{0x06}},
		},
		samples: []*media.Sample{
			{Data: []byte{0x02}, Samples: 1},
			{Data: []byte{0x03}, Samples: 1},
			{Data: []byte{0x04}, Samples: 1},
			{Data: []byte{0x05}, Samples: 1},
		},
		maxLate: 50,
	},
}

func TestSampleBuilder(t *testing.T) {
	assert := assert.New(t)

	for _, t := range testCases {
		s := New(t.maxLate, &fakeDepacketizer{})
		samples := []*media.Sample{}

		for _, p := range t.packets {
			s.Push(p)
		}
		for sample := s.Pop(); sample != nil; sample = s.Pop() {
			samples = append(samples, sample)
		}

		assert.Equal(samples, t.samples, t.message)
	}
}

// SampleBuilder should respect maxLate if we popped successfully but then have a gap larger then maxLate
func TestSampleBuilderMaxLate(t *testing.T) {
	assert := assert.New(t)
	s := New(50, &fakeDepacketizer{})

	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 0, Timestamp: 1}, Payload: []byte{0x01}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 1, Timestamp: 2}, Payload: []byte{0x01}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 2, Timestamp: 3}, Payload: []byte{0x01}})
	assert.Equal(s.Pop(), &media.Sample{Data: []byte{0x01}, Samples: 1}, "Failed to build samples before gap")

	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 500}, Payload: []byte{0x02}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 501}, Payload: []byte{0x02}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 502}, Payload: []byte{0x02}})
	assert.Equal(s.Pop(), &media.Sample{Data: []byte{0x02}, Samples: 1}, "Failed to build samples after large gap")
}
