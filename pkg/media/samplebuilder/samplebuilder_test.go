package samplebuilder

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
)

type sampleBuilderTest struct {
	message         string
	packets         []*rtp.Packet
	withHeadChecker bool
	headBytes       []byte
	samples         []*media.Sample
	timestamps      []uint32
	maxLate         uint16
}

type fakeDepacketizer struct {
}

func (f *fakeDepacketizer) Unmarshal(r []byte) ([]byte, error) {
	return r, nil
}

type fakePartitionHeadChecker struct {
	headBytes []byte
}

func (f *fakePartitionHeadChecker) IsPartitionHead(payload []byte) bool {
	for _, b := range f.headBytes {
		if payload[0] == b {
			return true
		}
	}
	return false
}

func TestSampleBuilder(t *testing.T) {
	testData := []sampleBuilderTest{
		{
			message: "SampleBuilder shouldn't emit anything if only one RTP packet has been pushed",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
			},
			samples:    []*media.Sample{},
			timestamps: []uint32{},
			maxLate:    50,
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
			timestamps: []uint32{
				6,
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
			timestamps: []uint32{
				6,
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
			samples:    []*media.Sample{},
			timestamps: []uint32{},
			maxLate:    50,
		},
		{
			message: "SampleBuilder should emit a packet after a gap if PartitionHeadChecker assumes it head",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
			},
			withHeadChecker: true,
			headBytes:       []byte{0x02},
			samples: []*media.Sample{
				{Data: []byte{0x02}, Samples: 0},
			},
			timestamps: []uint32{
				6,
			},
			maxLate: 50,
		},
		{
			message: "SampleBuilder shouldn't emit a packet after a gap if PartitionHeadChecker doesn't assume it head",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
			},
			withHeadChecker: true,
			headBytes:       []byte{},
			samples:         []*media.Sample{},
			timestamps:      []uint32{},
			maxLate:         50,
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
			timestamps: []uint32{
				2,
				3,
				4,
				5,
			},
			maxLate: 50,
		},
	}

	t.Run("Pop", func(t *testing.T) {
		assert := assert.New(t)

		for _, t := range testData {
			var opts []Option
			if t.withHeadChecker {
				opts = append(opts, WithPartitionHeadChecker(
					&fakePartitionHeadChecker{headBytes: t.headBytes},
				))
			}

			s := New(t.maxLate, &fakeDepacketizer{}, opts...)
			samples := []*media.Sample{}

			for _, p := range t.packets {
				s.Push(p)
			}
			for sample := s.Pop(); sample != nil; sample = s.Pop() {
				samples = append(samples, sample)
			}

			assert.Equal(samples, t.samples, t.message)
		}
	})
	t.Run("PopWithTimestamp", func(t *testing.T) {
		assert := assert.New(t)

		for _, t := range testData {
			var opts []Option
			if t.withHeadChecker {
				opts = append(opts, WithPartitionHeadChecker(
					&fakePartitionHeadChecker{headBytes: t.headBytes},
				))
			}

			s := New(t.maxLate, &fakeDepacketizer{}, opts...)
			samples := []*media.Sample{}
			timestamps := []uint32{}

			for _, p := range t.packets {
				s.Push(p)
			}
			for sample, timestamp := s.PopWithTimestamp(); sample != nil; sample, timestamp = s.PopWithTimestamp() {
				samples = append(samples, sample)
				timestamps = append(timestamps, timestamp)
			}

			assert.Equal(samples, t.samples, t.message)
			assert.Equal(timestamps, t.timestamps, t.message)
		}
	})
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

func TestSeqnumDistance(t *testing.T) {
	testData := []struct {
		x uint16
		y uint16
		d uint16
	}{
		{0x0001, 0x0003, 0x0002},
		{0x0003, 0x0001, 0x0002},
		{0xFFF3, 0xFFF1, 0x0002},
		{0xFFF1, 0xFFF3, 0x0002},
		{0xFFFF, 0x0001, 0x0002},
		{0x0001, 0xFFFF, 0x0002},
	}

	for _, data := range testData {
		if ret := seqnumDistance(data.x, data.y); ret != data.d {
			t.Errorf("seqnumDistance(%d, %d) returned %d which must be %d",
				data.x, data.y, ret, data.d)
		}
	}
}

func TestSampleBuilderCleanReference(t *testing.T) {
	s := New(10, &fakeDepacketizer{})

	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 0, Timestamp: 0}, Payload: []byte{0x01}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 1, Timestamp: 0}, Payload: []byte{0x02}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 2, Timestamp: 0}, Payload: []byte{0x03}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 13, Timestamp: 120}, Payload: []byte{0x04}})

	for i := 0; i < 3; i++ {
		if s.buffer[i] != nil {
			t.Errorf("Old packet (%d) is not unreferenced (maxLate: 10, pushed: 12)", i)
		}
	}
}
