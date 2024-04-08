// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package samplebuilder

import (
	"fmt"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/stretchr/testify/assert"
)

type sampleBuilderTest struct {
	message          string
	packets          []*rtp.Packet
	withHeadChecker  bool
	withRTPHeader    bool
	headBytes        []byte
	samples          []*media.Sample
	maxLate          uint16
	maxLateTimestamp uint32
}

type fakeDepacketizer struct {
	headChecker bool
	headBytes   []byte
	alwaysHead  bool
}

func (f *fakeDepacketizer) Unmarshal(r []byte) ([]byte, error) {
	return r, nil
}

func (f *fakeDepacketizer) IsPartitionHead(payload []byte) bool {
	if !f.headChecker {
		// simulates a bug in the 3.0 version
		// the tests should be fixed to not assume the bug
		return true
	}

	// skip padding
	if len(payload) < 1 {
		return false
	}

	if f.alwaysHead {
		return true
	}

	for _, b := range f.headBytes {
		if payload[0] == b {
			return true
		}
	}
	return false
}

func (f *fakeDepacketizer) IsPartitionTail(marker bool, _ []byte) bool {
	return marker
}

func TestSampleBuilder(t *testing.T) {
	testData := []sampleBuilderTest{
		{
			message: "SampleBuilder shouldn't emit anything if only one RTP packet has been pushed",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
			},
			samples:          []*media.Sample{},
			maxLate:          50,
			maxLateTimestamp: 0,
		},
		{
			message: "SampleBuilder shouldn't emit anything if only one RTP packet has been pushed even if the market bit is set",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5, Marker: true}, Payload: []byte{0x01}},
			},
			samples:          []*media.Sample{},
			maxLate:          50,
			maxLateTimestamp: 0,
		},
		{
			message: "SampleBuilder should emit two packets, we had three packets with unique timestamps",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 7}, Payload: []byte{0x03}},
			},
			samples: []*media.Sample{
				{Data: []byte{0x01}, Duration: time.Second, PacketTimestamp: 5},
				{Data: []byte{0x02}, Duration: time.Second, PacketTimestamp: 6},
			},
			maxLate:          50,
			maxLateTimestamp: 0,
		},
		{
			message: "SampleBuilder should emit one packet, we had a packet end of sequence marker and run out of space",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5, Marker: true}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 7}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5004, Timestamp: 9}, Payload: []byte{0x03}},
				{Header: rtp.Header{SequenceNumber: 5006, Timestamp: 11}, Payload: []byte{0x04}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 13}, Payload: []byte{0x05}},
				{Header: rtp.Header{SequenceNumber: 5010, Timestamp: 15}, Payload: []byte{0x06}},
				{Header: rtp.Header{SequenceNumber: 5012, Timestamp: 17}, Payload: []byte{0x07}},
			},
			samples: []*media.Sample{
				{Data: []byte{0x01}, Duration: time.Second * 2, PacketTimestamp: 5},
			},
			maxLate:          5,
			maxLateTimestamp: 0,
		},
		{
			message: "SampleBuilder shouldn't emit any packet, we do not have a valid end of sequence and run out of space",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 7}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5004, Timestamp: 9}, Payload: []byte{0x03}},
				{Header: rtp.Header{SequenceNumber: 5006, Timestamp: 11}, Payload: []byte{0x04}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 13}, Payload: []byte{0x05}},
				{Header: rtp.Header{SequenceNumber: 5010, Timestamp: 15}, Payload: []byte{0x06}},
				{Header: rtp.Header{SequenceNumber: 5012, Timestamp: 17}, Payload: []byte{0x07}},
			},
			samples:          []*media.Sample{},
			maxLate:          5,
			maxLateTimestamp: 0,
		},
		{
			message: "SampleBuilder should emit one packet, we had a packet end of sequence marker and run out of space",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5, Marker: true}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 7, Marker: true}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5004, Timestamp: 9}, Payload: []byte{0x03}},
				{Header: rtp.Header{SequenceNumber: 5006, Timestamp: 11}, Payload: []byte{0x04}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 13}, Payload: []byte{0x05}},
				{Header: rtp.Header{SequenceNumber: 5010, Timestamp: 15}, Payload: []byte{0x06}},
				{Header: rtp.Header{SequenceNumber: 5012, Timestamp: 17}, Payload: []byte{0x07}},
			},
			samples: []*media.Sample{
				{Data: []byte{0x01}, Duration: time.Second * 2, PacketTimestamp: 5},
				{Data: []byte{0x02}, Duration: time.Second * 2, PacketTimestamp: 7, PrevDroppedPackets: 1},
			},
			maxLate:          5,
			maxLateTimestamp: 0,
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
				{Data: []byte{0x01}, Duration: time.Second, PacketTimestamp: 5},
				{Data: []byte{0x02, 0x03}, Duration: time.Second, PacketTimestamp: 6},
			},
			maxLate:          50,
			maxLateTimestamp: 0,
		},
		{
			message: "SampleBuilder shouldn't emit a packet because we have a gap before a valid one",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
			},
			samples:          []*media.Sample{},
			maxLate:          50,
			maxLateTimestamp: 0,
		},
		{
			message: "SampleBuilder shouldn't emit a packet after a gap as there are gaps and have not reached maxLate yet",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
			},
			withHeadChecker:  true,
			headBytes:        []byte{0x02},
			samples:          []*media.Sample{},
			maxLate:          50,
			maxLateTimestamp: 0,
		},
		{
			message: "SampleBuilder shouldn't emit a packet after a gap if PartitionHeadChecker doesn't assume it head",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
			},
			withHeadChecker:  true,
			headBytes:        []byte{},
			samples:          []*media.Sample{},
			maxLate:          50,
			maxLateTimestamp: 0,
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
				{Data: []byte{0x01}, Duration: time.Second, PacketTimestamp: 1},
				{Data: []byte{0x02}, Duration: time.Second, PacketTimestamp: 2},
				{Data: []byte{0x03}, Duration: time.Second, PacketTimestamp: 3},
				{Data: []byte{0x04}, Duration: time.Second, PacketTimestamp: 4},
				{Data: []byte{0x05}, Duration: time.Second, PacketTimestamp: 5},
			},
			maxLate:          50,
			maxLateTimestamp: 0,
		},
		{
			message: "SampleBuilder should skip time stamps too old",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 1}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 2}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 3}, Payload: []byte{0x03}},
				{Header: rtp.Header{SequenceNumber: 5013, Timestamp: 4000}, Payload: []byte{0x04}},
				{Header: rtp.Header{SequenceNumber: 5014, Timestamp: 4000}, Payload: []byte{0x05}},
				{Header: rtp.Header{SequenceNumber: 5015, Timestamp: 4002}, Payload: []byte{0x06}},
				{Header: rtp.Header{SequenceNumber: 5016, Timestamp: 7000}, Payload: []byte{0x04}},
				{Header: rtp.Header{SequenceNumber: 5017, Timestamp: 7001}, Payload: []byte{0x05}},
			},
			samples: []*media.Sample{
				{Data: []byte{0x04, 0x05}, Duration: time.Second * time.Duration(2), PacketTimestamp: 4000, PrevDroppedPackets: 13},
			},
			withHeadChecker:  true,
			headBytes:        []byte{0x04},
			maxLate:          50,
			maxLateTimestamp: 2000,
		},
		{
			message: "Sample builder should recognize padding packets",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 1}, Payload: []byte{1}},               // 1st packet
				{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 1}, Payload: []byte{2}},               // 2nd packet
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 1, Marker: true}, Payload: []byte{3}}, // 3rd packet
				{Header: rtp.Header{SequenceNumber: 5003, Timestamp: 1}, Payload: []byte{}},                // Padding packet 1
				{Header: rtp.Header{SequenceNumber: 5004, Timestamp: 1}, Payload: []byte{}},                // Padding packet 2
				{Header: rtp.Header{SequenceNumber: 5005, Timestamp: 3}, Payload: []byte{1}},               // 6th packet
				{Header: rtp.Header{SequenceNumber: 5006, Timestamp: 3, Marker: true}, Payload: []byte{7}}, // 7th packet
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 4}, Payload: []byte{1}},               // 7th packet
			},
			withHeadChecker: true,
			headBytes:       []byte{1},
			samples: []*media.Sample{
				{Data: []byte{1, 2, 3}, Duration: 0, PacketTimestamp: 1, PrevDroppedPackets: 0}, // first sample
			},
			maxLate:          50,
			maxLateTimestamp: 2000,
		},
		{
			message: "Sample builder should build a sample out of a packet that's both start and end following a run of padding packets",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 1}, Payload: []byte{1}},               // 1st valid packet
				{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 1, Marker: true}, Payload: []byte{2}}, // 2nd valid packet
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 1}, Payload: []byte{}},                // 1st padding packet
				{Header: rtp.Header{SequenceNumber: 5003, Timestamp: 1}, Payload: []byte{}},                // 2nd padding packet
				{Header: rtp.Header{SequenceNumber: 5004, Timestamp: 2, Marker: true}, Payload: []byte{1}}, // 3rd valid packet
				{Header: rtp.Header{SequenceNumber: 5005, Timestamp: 3}, Payload: []byte{1}},               // 4th valid packet, start of next sample
			},
			withHeadChecker: true,
			headBytes:       []byte{1},
			samples: []*media.Sample{
				{Data: []byte{1, 2}, Duration: 0, PacketTimestamp: 1, PrevDroppedPackets: 0}, // 1st sample
			},
			maxLate:          50,
			maxLateTimestamp: 2000,
		},
		{
			message: "SampleBuilder should emit samples with RTP headers when WithRTPHeaders option is enabled",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 6}, Payload: []byte{0x03}},
				{Header: rtp.Header{SequenceNumber: 5003, Timestamp: 7}, Payload: []byte{0x04}},
			},
			samples: []*media.Sample{
				{Data: []byte{0x01}, Duration: time.Second, PacketTimestamp: 5, RTPHeaders: []*rtp.Header{
					{SequenceNumber: 5000, Timestamp: 5},
				}},
				{Data: []byte{0x02, 0x03}, Duration: time.Second, PacketTimestamp: 6, RTPHeaders: []*rtp.Header{
					{SequenceNumber: 5001, Timestamp: 6},
					{SequenceNumber: 5002, Timestamp: 6},
				}},
			},
			maxLate:          50,
			maxLateTimestamp: 0,
			withRTPHeader:    true,
		},
	}

	t.Run("Pop", func(t *testing.T) {
		assert := assert.New(t)

		for _, t := range testData {
			var opts []Option
			if t.maxLateTimestamp != 0 {
				opts = append(opts, WithMaxTimeDelay(
					time.Millisecond*time.Duration(int64(t.maxLateTimestamp)),
				))
			}
			if t.withRTPHeader {
				opts = append(opts, WithRTPHeaders(true))
			}

			d := &fakeDepacketizer{
				headChecker: t.withHeadChecker,
				headBytes:   t.headBytes,
			}
			s := New(t.maxLate, d, 1, opts...)
			samples := []*media.Sample{}

			for _, p := range t.packets {
				s.Push(p)
			}
			for sample := s.Pop(); sample != nil; sample = s.Pop() {
				samples = append(samples, sample)
			}
			assert.Equal(t.samples, samples, t.message)
		}
	})
}

// SampleBuilder should respect maxLate if we popped successfully but then have a gap larger then maxLate
func TestSampleBuilderMaxLate(t *testing.T) {
	assert := assert.New(t)
	s := New(50, &fakeDepacketizer{}, 1)

	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 0, Timestamp: 1}, Payload: []byte{0x01}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 1, Timestamp: 2}, Payload: []byte{0x01}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 2, Timestamp: 3}, Payload: []byte{0x01}})
	assert.Equal(&media.Sample{Data: []byte{0x01}, Duration: time.Second, PacketTimestamp: 1}, s.Pop(), "Failed to build samples before gap")

	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 500}, Payload: []byte{0x02}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 501}, Payload: []byte{0x02}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 502}, Payload: []byte{0x02}})

	assert.Equal(&media.Sample{Data: []byte{0x01}, Duration: time.Second, PacketTimestamp: 2}, s.Pop(), "Failed to build samples after large gap")
	assert.Equal((*media.Sample)(nil), s.Pop(), "Failed to build samples after large gap")

	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 6000, Timestamp: 600}, Payload: []byte{0x03}})
	assert.Equal(&media.Sample{Data: []byte{0x02}, Duration: time.Second, PacketTimestamp: 500, PrevDroppedPackets: 4998}, s.Pop(), "Failed to build samples after large gap")
	assert.Equal(&media.Sample{Data: []byte{0x02}, Duration: time.Second, PacketTimestamp: 501}, s.Pop(), "Failed to build samples after large gap")
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
	for _, seqStart := range []uint16{
		0,
		0xFFF8, // check upper boundary
		0xFFFE, // check upper boundary
	} {
		seqStart := seqStart
		t.Run(fmt.Sprintf("From%d", seqStart), func(t *testing.T) {
			s := New(10, &fakeDepacketizer{}, 1)

			s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 0 + seqStart, Timestamp: 0}, Payload: []byte{0x01}})
			s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 1 + seqStart, Timestamp: 0}, Payload: []byte{0x02}})
			s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 2 + seqStart, Timestamp: 0}, Payload: []byte{0x03}})
			pkt4 := &rtp.Packet{Header: rtp.Header{SequenceNumber: 14 + seqStart, Timestamp: 120}, Payload: []byte{0x04}}
			s.Push(pkt4)
			pkt5 := &rtp.Packet{Header: rtp.Header{SequenceNumber: 12 + seqStart, Timestamp: 120}, Payload: []byte{0x05}}
			s.Push(pkt5)

			for i := 0; i < 3; i++ {
				pkt, err := s.buffer.PeekAtSequence(uint16((i + int(seqStart)) % 0x10000))

				if pkt != nil || err == nil {
					t.Errorf("Old packet (%d) is not unreferenced (maxLate: 10, pushed: 12)", i)
				}
			}
			pkt, err := s.buffer.PeekAtSequence(uint16((14 + int(seqStart)) % 0x10000))
			if pkt != pkt4 || err != nil {
				t.Error("New packet must be referenced after jump")
			}
			pkt, err = s.buffer.PeekAtSequence(uint16((12 + int(seqStart)) % 0x10000))
			if pkt != pkt5 || err != nil {
				t.Error("New packet must be referenced after jump")
			}
		})
	}
}

func TestSampleBuilderPushMaxZero(t *testing.T) {
	// Test packets released via 'maxLate' of zero.
	pkts := []rtp.Packet{
		{Header: rtp.Header{SequenceNumber: 0, Timestamp: 0, Marker: true}, Payload: []byte{0x01}},
	}
	d := &fakeDepacketizer{
		headChecker: true,
		headBytes:   []byte{0x01},
	}

	s := New(0, d, 1)
	s.Push(&pkts[0])
	if sample := s.Pop(); sample == nil {
		t.Error("Should expect a popped sample")
	}
}

func TestSampleBuilderWithPacketReleaseHandler(t *testing.T) {
	var released []*rtp.Packet
	fakePacketReleaseHandler := func(p *rtp.Packet) {
		released = append(released, p)
	}

	// Test packets released via 'maxLate'.
	pkts := []rtp.Packet{
		{Header: rtp.Header{SequenceNumber: 0, Timestamp: 0}, Payload: []byte{0x01}},
		{Header: rtp.Header{SequenceNumber: 11, Timestamp: 120}, Payload: []byte{0x02}},
		{Header: rtp.Header{SequenceNumber: 12, Timestamp: 121}, Payload: []byte{0x03}},
		{Header: rtp.Header{SequenceNumber: 13, Timestamp: 122}, Payload: []byte{0x04}},
		{Header: rtp.Header{SequenceNumber: 21, Timestamp: 200}, Payload: []byte{0x05}},
	}
	s := New(10, &fakeDepacketizer{}, 1, WithPacketReleaseHandler(fakePacketReleaseHandler))
	s.Push(&pkts[0])
	s.Push(&pkts[1])
	if len(released) == 0 {
		t.Errorf("Old packet is not released")
	}
	if len(released) > 0 && released[0].SequenceNumber != pkts[0].SequenceNumber {
		t.Errorf("Unexpected packet released by maxLate")
	}
	// Test packets released after samples built.
	s.Push(&pkts[2])
	s.Push(&pkts[3])
	s.Push(&pkts[4])
	if s.Pop() == nil {
		t.Errorf("Should have some sample here.")
	}
	if len(released) < 3 {
		t.Errorf("packet built with sample is not released")
	}
	if len(released) >= 2 && released[2].SequenceNumber != pkts[2].SequenceNumber {
		t.Errorf("Unexpected packet released by samples built")
	}
}

func TestSampleBuilderWithPacketHeadHandler(t *testing.T) {
	packets := []*rtp.Packet{
		{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
		{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 5}, Payload: []byte{0x02}},
		{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 6}, Payload: []byte{0x01}},
		{Header: rtp.Header{SequenceNumber: 5003, Timestamp: 6}, Payload: []byte{0x02}},
		{Header: rtp.Header{SequenceNumber: 5004, Timestamp: 7}, Payload: []byte{0x01}},
	}

	headCount := 0
	s := New(10, &fakeDepacketizer{}, 1, WithPacketHeadHandler(func(interface{}) interface{} {
		headCount++
		return true
	}))

	for _, pkt := range packets {
		s.Push(pkt)
	}

	for {
		sample := s.Pop()
		if sample == nil {
			break
		}

		assert.NotNil(t, sample.Metadata, "sample metadata shouldn't be nil")
		assert.Equal(t, true, sample.Metadata, "sample metadata should've been set to true")
	}

	assert.Equal(t, 2, headCount, "two sample heads should have been inspected")
}

func TestSampleBuilderData(t *testing.T) {
	s := New(10, &fakeDepacketizer{
		headChecker: true,
		alwaysHead:  true,
	}, 1)
	j := 0
	for i := 0; i < 0x20000; i++ {
		p := rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: uint16(i),
				Timestamp:      uint32(i + 42),
			},
			Payload: []byte{byte(i)},
		}
		s.Push(&p)
		for {
			sample := s.Pop()
			if sample == nil {
				break
			}
			assert.Equal(t, sample.PacketTimestamp, uint32(j+42), "timestamp")
			assert.Equal(t, len(sample.Data), 1, "data length")
			assert.Equal(t, byte(j), sample.Data[0], "data")
			j++
		}
	}
	// only the last packet should be dropped
	assert.Equal(t, j, 0x1FFFF)
}

func TestSampleBuilder_Flush(t *testing.T) {
	s := New(50, &fakeDepacketizer{
		headChecker: true,
		headBytes:   []byte{0x01},
	}, 1)

	s.Push(&rtp.Packet{
		Header:  rtp.Header{SequenceNumber: 999, Timestamp: 0},
		Payload: []byte{0x00},
	}) // Invalid packet
	// Gap preventing below packets to be processed
	s.Push(&rtp.Packet{
		Header:  rtp.Header{SequenceNumber: 1001, Timestamp: 1, Marker: true},
		Payload: []byte{0x01, 0x11},
	}) // Valid packet
	s.Push(&rtp.Packet{
		Header:  rtp.Header{SequenceNumber: 1011, Timestamp: 10, Marker: true},
		Payload: []byte{0x01, 0x12},
	}) // Valid packet

	if sample := s.Pop(); sample != nil {
		t.Fatal("Unexpected sample is returned. Test precondition may be broken")
	}

	s.Flush()

	samples := []*media.Sample{}
	for sample := s.Pop(); sample != nil; sample = s.Pop() {
		samples = append(samples, sample)
	}

	expected := []*media.Sample{
		{Data: []byte{0x01, 0x11}, Duration: 9 * time.Second, PacketTimestamp: 1, PrevDroppedPackets: 2},
		{Data: []byte{0x01, 0x12}, Duration: 0, PacketTimestamp: 10, PrevDroppedPackets: 9},
	}

	assert.Equal(t, expected, samples)
}

func BenchmarkSampleBuilderSequential(b *testing.B) {
	s := New(100, &fakeDepacketizer{}, 1)
	b.ResetTimer()
	j := 0
	for i := 0; i < b.N; i++ {
		p := rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: uint16(i),
				Timestamp:      uint32(i + 42),
			},
			Payload: make([]byte, 50),
		}
		s.Push(&p)
		for {
			s := s.Pop()
			if s == nil {
				break
			}
			j++
		}
	}
	if b.N > 200 && j < b.N-100 {
		b.Errorf("Got %v (N=%v)", j, b.N)
	}
}

func BenchmarkSampleBuilderLoss(b *testing.B) {
	s := New(100, &fakeDepacketizer{}, 1)
	b.ResetTimer()
	j := 0
	for i := 0; i < b.N; i++ {
		if i%13 == 0 {
			continue
		}
		p := rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: uint16(i),
				Timestamp:      uint32(i + 42),
			},
			Payload: make([]byte, 50),
		}
		s.Push(&p)
		for {
			s := s.Pop()
			if s == nil {
				break
			}
			j++
		}
	}
	if b.N > 200 && j < b.N/2-100 {
		b.Errorf("Got %v (N=%v)", j, b.N)
	}
}

func BenchmarkSampleBuilderReordered(b *testing.B) {
	s := New(100, &fakeDepacketizer{}, 1)
	b.ResetTimer()
	j := 0
	for i := 0; i < b.N; i++ {
		p := rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: uint16(i ^ 3),
				Timestamp:      uint32((i ^ 3) + 42),
			},
			Payload: make([]byte, 50),
		}
		s.Push(&p)
		for {
			s := s.Pop()
			if s == nil {
				break
			}
			j++
		}
	}
	if b.N > 2 && j < b.N-5 && j > b.N {
		b.Errorf("Got %v (N=%v)", j, b.N)
	}
}

func BenchmarkSampleBuilderFragmented(b *testing.B) {
	s := New(100, &fakeDepacketizer{}, 1)
	b.ResetTimer()
	j := 0
	for i := 0; i < b.N; i++ {
		p := rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: uint16(i),
				Timestamp:      uint32(i/2 + 42),
			},
			Payload: make([]byte, 50),
		}
		s.Push(&p)
		for {
			s := s.Pop()
			if s == nil {
				break
			}
			j++
		}
	}
	if b.N > 200 && j < b.N/2-100 {
		b.Errorf("Got %v (N=%v)", j, b.N)
	}
}

func BenchmarkSampleBuilderFragmentedLoss(b *testing.B) {
	s := New(100, &fakeDepacketizer{}, 1)
	b.ResetTimer()
	j := 0
	for i := 0; i < b.N; i++ {
		if i%13 == 0 {
			continue
		}
		p := rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: uint16(i),
				Timestamp:      uint32(i/2 + 42),
			},
			Payload: make([]byte, 50),
		}
		s.Push(&p)
		for {
			s := s.Pop()
			if s == nil {
				break
			}
			j++
		}
	}
	if b.N > 200 && j < b.N/3-100 {
		b.Errorf("Got %v (N=%v)", j, b.N)
	}
}
