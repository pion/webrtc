// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/logging"
	"github.com/pion/transport/v3/test"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Assert that SetReadDeadline works as expected
// This test uses VNet since we must have zero loss.
func Test_RTPReceiver_SetReadDeadline(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	sender, receiver, wan := createVNetPair(t, &interceptor.Registry{})

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = sender.AddTrack(track)
	assert.NoError(t, err)

	seenPacket, seenPacketCancel := context.WithCancel(context.Background())
	receiver.OnTrack(func(trackRemote *TrackRemote, r *RTPReceiver) {
		// Set Deadline for both RTP and RTCP Stream
		assert.NoError(t, r.SetReadDeadline(time.Now().Add(time.Second)))
		assert.NoError(t, trackRemote.SetReadDeadline(time.Now().Add(time.Second)))

		// First call will not error because we cache for probing
		_, _, readErr := trackRemote.ReadRTP()
		assert.NoError(t, readErr)

		_, _, readErr = trackRemote.ReadRTP()
		assert.Error(t, readErr)

		_, _, readErr = r.ReadRTCP()
		assert.Error(t, readErr)

		seenPacketCancel()
	})

	peerConnectionsConnected := untilConnectionState(PeerConnectionStateConnected, sender, receiver)

	assert.NoError(t, signalPair(sender, receiver))

	peerConnectionsConnected.Wait()
	assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0xAA}, Duration: time.Second}))

	<-seenPacket.Done()
	assert.NoError(t, wan.Stop())
	closePairNow(t, sender, receiver)
}

// TestRTPReceiver_CollectStats_Mapping validates that collectStats maps
// interceptor/pkg/stats values into InboundRTPStreamStats.
func TestRTPReceiver_CollectStats_Mapping(t *testing.T) {
	ssrc := SSRC(1234)
	now := time.Now()
	pr := uint64(math.MaxUint32) + 42
	pl := int64(math.MaxInt32) + 7
	jitter := 0.123
	bytes := uint64(98765)
	hdrBytes := uint64(4321)
	fir := uint32(3)
	pli := uint32(5)
	nack := uint32(7)

	fg := &fakeGetter{s: stats.Stats{
		InboundRTPStreamStats: stats.InboundRTPStreamStats{
			ReceivedRTPStreamStats: stats.ReceivedRTPStreamStats{
				PacketsReceived: pr,
				PacketsLost:     pl,
				Jitter:          jitter,
			},
			LastPacketReceivedTimestamp: now,
			HeaderBytesReceived:         hdrBytes,
			BytesReceived:               bytes,
			FIRCount:                    fir,
			PLICount:                    pli,
			NACKCount:                   nack,
		},
	}}

	// Minimal RTPReceiver with one track
	r := &RTPReceiver{
		kind: RTPCodecTypeVideo,
		log:  logging.NewDefaultLoggerFactory().NewLogger("RTPReceiverTest"),
	}
	tr := newTrackRemote(RTPCodecTypeVideo, ssrc, 0, "", r)
	r.tracks = []trackStreams{{track: tr}}

	collector := newStatsReportCollector()
	r.collectStats(collector, fg)
	report := collector.Ready()

	// Fetch the generated inbound-rtp stat by ID
	statID := "inbound-rtp-1234"
	got, ok := report[statID]
	require.True(t, ok, "missing inbound stat")

	inbound, ok := got.(InboundRTPStreamStats)
	require.True(t, ok)

	// Wrap-around semantics for casts
	assert.Equal(t, uint32(pr), inbound.PacketsReceived) //nolint:gosec
	assert.Equal(t, int32(pl), inbound.PacketsLost)      //nolint:gosec
	assert.Equal(t, jitter, inbound.Jitter)
	assert.Equal(t, bytes, inbound.BytesReceived)
	assert.Equal(t, hdrBytes, inbound.HeaderBytesReceived)
	assert.Equal(t, fir, inbound.FIRCount)
	assert.Equal(t, pli, inbound.PLICount)
	assert.Equal(t, nack, inbound.NACKCount)
	// Timestamp should be set (millisecond precision)
	assert.Greater(t, float64(inbound.LastPacketReceivedTimestamp), 0.0)
}

type fakeGetter struct{ s stats.Stats }

func (f *fakeGetter) Get(uint32) *stats.Stats { return &f.s }
