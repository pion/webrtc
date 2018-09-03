package samplebuilder

import (
	"testing"

	"github.com/pions/webrtc/pkg/media"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/stretchr/testify/assert"
)

type sampleBuilderTest struct {
	message string
	packets []*rtp.Packet
	samples []*media.RTCSample
}

var testCases = []sampleBuilderTest{
	sampleBuilderTest{
		message: "SampleBuilder shouldn't emit anything if only one RTP packet has been pushed",
		packets: []*rtp.Packet{
			&rtp.Packet{SequenceNumber: 5000, Timestamp: 5, Payload: []byte{0x01}},
		},
		samples: []*media.RTCSample{},
	},
	sampleBuilderTest{
		message: "SampleBuilder should emit one packet, we had three packets with unique timestamps",
		packets: []*rtp.Packet{
			&rtp.Packet{SequenceNumber: 5000, Timestamp: 5, Payload: []byte{0x01}},
			&rtp.Packet{SequenceNumber: 5001, Timestamp: 6, Payload: []byte{0x02}},
			&rtp.Packet{SequenceNumber: 5002, Timestamp: 7, Payload: []byte{0x03}},
		},
		samples: []*media.RTCSample{
			&media.RTCSample{Data: []byte{0x02}},
		},
	},
	sampleBuilderTest{
		message: "SampleBuilder should emit one packet, we had two packets but two with duplicate timestamps",
		packets: []*rtp.Packet{
			&rtp.Packet{SequenceNumber: 5000, Timestamp: 5, Payload: []byte{0x01}},
			&rtp.Packet{SequenceNumber: 5001, Timestamp: 6, Payload: []byte{0x02}},
			&rtp.Packet{SequenceNumber: 5002, Timestamp: 6, Payload: []byte{0x03}},
			&rtp.Packet{SequenceNumber: 5003, Timestamp: 7, Payload: []byte{0x04}},
		},
		samples: []*media.RTCSample{
			&media.RTCSample{Data: []byte{0x02, 0x03}},
		},
	},
	sampleBuilderTest{
		message: "SampleBuilder shouldn't emit a packet because we have a gap before a valid one",
		packets: []*rtp.Packet{
			&rtp.Packet{SequenceNumber: 5000, Timestamp: 5, Payload: []byte{0x01}},
			&rtp.Packet{SequenceNumber: 5007, Timestamp: 6, Payload: []byte{0x02}},
			&rtp.Packet{SequenceNumber: 5008, Timestamp: 7, Payload: []byte{0x03}},
		},
		samples: []*media.RTCSample{},
	},
	sampleBuilderTest{
		message: "SampleBuilder shouldn't emit multiple valid packets",
		packets: []*rtp.Packet{
			&rtp.Packet{SequenceNumber: 5000, Timestamp: 1, Payload: []byte{0x01}},
			&rtp.Packet{SequenceNumber: 5001, Timestamp: 2, Payload: []byte{0x02}},
			&rtp.Packet{SequenceNumber: 5002, Timestamp: 3, Payload: []byte{0x03}},
			&rtp.Packet{SequenceNumber: 5003, Timestamp: 4, Payload: []byte{0x04}},
			&rtp.Packet{SequenceNumber: 5004, Timestamp: 5, Payload: []byte{0x05}},
			&rtp.Packet{SequenceNumber: 5005, Timestamp: 6, Payload: []byte{0x06}},
		},
		samples: []*media.RTCSample{
			&media.RTCSample{Data: []byte{0x02}},
			&media.RTCSample{Data: []byte{0x03}},
			&media.RTCSample{Data: []byte{0x04}},
			&media.RTCSample{Data: []byte{0x05}},
		},
	},
}

func TestSampleBuilder(t *testing.T) {
	assert := assert.New(t)

	for _, t := range testCases {
		s := New(50, 90000)
		samples := []*media.RTCSample{}

		for _, p := range t.packets {
			s.Push(p)
		}
		for sample := s.Pop(); sample != nil; sample = s.Pop() {
			samples = append(samples, sample)
		}

		assert.Equal(samples, t.samples, t.message)
	}
}
