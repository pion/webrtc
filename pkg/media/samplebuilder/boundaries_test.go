// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package samplebuilder

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSampleBuilderDurationAcrossSequenceWrap(t *testing.T) {
	for _, sequence := range []uint16{1, 65533, 65534, 65535} {
		for _, timestamp := range []uint32{1000, 0xfffffe1f} {
			t.Run(fmt.Sprintf("seq%d_ts%d", sequence, timestamp), func(t *testing.T) {
				builder := New(50, &fakeDepacketizer{}, 48000)
				builder.Push(&rtp.Packet{
					Header: rtp.Header{SequenceNumber: sequence, Timestamp: timestamp}, Payload: []byte{1},
				})
				builder.Push(&rtp.Packet{
					Header: rtp.Header{SequenceNumber: sequence + 1, Timestamp: timestamp, Marker: true}, Payload: []byte{2},
				})
				builder.Push(&rtp.Packet{
					Header: rtp.Header{SequenceNumber: sequence + 2, Timestamp: timestamp + 960}, Payload: []byte{3},
				})
				sample := builder.Pop()
				require.NotNil(t, sample)
				assert.Equal(t, []byte{1, 2}, sample.Data)
				assert.Equal(t, timestamp, sample.PacketTimestamp)
				assert.Equal(t, 20*time.Millisecond, sample.Duration)
			})
		}
	}
}

func TestSampleBuilderReleaseWithLossAndReordering(t *testing.T) {
	released := make(map[*rtp.Packet]int)
	builder := New(12, &fakeDepacketizer{headChecker: true, headBytes: []byte{1}}, 48000,
		WithRTPHeaders(true), WithPacketReleaseHandler(func(packet *rtp.Packet) {
			released[packet]++
			clear(packet.Payload)
			packet.Timestamp = 0
		}))
	sequence := uint16(65530)
	want := make(map[uint32][]byte)
	var packets []*rtp.Packet
	for frame := range uint32(20) {
		timestamp := frame * 960
		var data []byte
		for fragment := range uint16(3) {
			payload := bytes.Repeat([]byte{byte(fragment + 1)}, 1000)
			data = append(data, payload...)
			packet := &rtp.Packet{
				Header: rtp.Header{SequenceNumber: sequence, Timestamp: timestamp, Marker: fragment == 2}, Payload: payload,
			}
			sequence++
			if frame%7 == 3 && fragment == 1 {
				continue
			}
			packets = append(packets, packet)
		}
		if frame%7 != 3 {
			want[timestamp] = data
		}
	}
	// Reverse pairs to exercise reordering across frame and sequence boundaries.
	for index := 0; index+1 < len(packets); index += 2 {
		packets[index], packets[index+1] = packets[index+1], packets[index]
	}
	for _, packet := range packets {
		builder.Push(packet)
	}
	builder.Flush()
	for sample := builder.Pop(); sample != nil; sample = builder.Pop() {
		data, exists := want[sample.PacketTimestamp]
		require.True(t, exists, "unexpected or duplicate frame %d", sample.PacketTimestamp)
		require.Equal(t, data, sample.Data, "released packet storage must not corrupt samples")
		require.Len(t, sample.RTPHeaders, 3)
		require.Equal(t, sample.PacketTimestamp, sample.RTPHeaders[0].Timestamp)
		require.Equal(t, sample.PacketTimestamp, sample.RTPHeaders[1].Timestamp)
		require.Equal(t, sample.PacketTimestamp, sample.RTPHeaders[2].Timestamp)
		delete(want, sample.PacketTimestamp)
	}
	require.Zero(t, len(want), "all complete frames should be emitted")
	for _, packet := range packets {
		require.Equal(t, 1, released[packet], "each retained packet must be released exactly once")
	}
}

var errRejectedFragment = errors.New("invalid sample fragment")

type rejectingDepacketizer struct{ fakeDepacketizer }

func (f *rejectingDepacketizer) Unmarshal(payload []byte) ([]byte, error) {
	if payload[0] == 0xee {
		return nil, errRejectedFragment
	}

	return payload, nil
}

func TestSampleBuilderInvalidFragmentThenValidSample(t *testing.T) {
	for _, badFragment := range []int{0, 1} {
		t.Run(fmt.Sprint(badFragment), func(t *testing.T) {
			released := 0
			builder := New(50, &rejectingDepacketizer{}, 1,
				WithPacketReleaseHandler(func(*rtp.Packet) { released++ }))
			for index := range 4 {
				payload := []byte{1}
				if index == badFragment {
					payload[0] = 0xee
				}
				builder.Push(&rtp.Packet{
					Header: rtp.Header{
						SequenceNumber: uint16(1000 + index),  //nolint:gosec // bounded by the four-packet fixture
						Timestamp:      uint32(max(1, index)), //nolint:gosec // bounded by the four-packet fixture
						Marker:         index == 1 || index == 2,
					},
					Payload: payload,
				})
			}
			builder.Flush()
			sample := builder.Pop()
			require.NotNil(t, sample)
			require.Equal(t, uint32(2), sample.PacketTimestamp)
			require.Equal(t, uint16(2), sample.PrevDroppedPackets)
			require.Equal(t, []byte{1}, sample.Data)
			require.Nil(t, builder.Pop())
			require.Equal(t, 4, released)
		})
	}
}
