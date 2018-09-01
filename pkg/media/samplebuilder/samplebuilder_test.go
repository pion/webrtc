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
