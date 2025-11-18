// SPDX-FileCopyrightText: 2024 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTrackAudioPlayoutStatsProvider struct {
	stats AudioPlayoutStats
	ok    bool

	calls   int
	lastNow time.Time
}

func (f *fakeTrackAudioPlayoutStatsProvider) Snapshot(now time.Time) (AudioPlayoutStats, bool) {
	f.calls++
	f.lastNow = now

	return f.stats, f.ok
}

func (f *fakeTrackAudioPlayoutStatsProvider) AddTrack(track *TrackRemote) error {
	track.addProvider(f)

	return nil
}

func (f *fakeTrackAudioPlayoutStatsProvider) RemoveTrack(track *TrackRemote) {
	track.removeProvider(f)
}

func TestTrackRemotePullAudioPlayoutStats(t *testing.T) {
	receiver := &RTPReceiver{}
	track := newTrackRemote(RTPCodecTypeAudio, 4242, 0, "", receiver)

	provider := &fakeTrackAudioPlayoutStatsProvider{
		stats: AudioPlayoutStats{
			ID:                "media-playout-4242",
			Type:              StatsTypeMediaPlayout,
			Kind:              string(MediaKindAudio),
			TotalSamplesCount: 960,
		},
		ok: true,
	}

	err := provider.AddTrack(track)
	require.NoError(t, err)

	now := time.Unix(1710000000, 0)
	allStats := track.pullAudioPlayoutStats(now)

	require.Len(t, allStats, 1)
	stats := allStats[0]
	assert.Equal(t, provider.stats.TotalSamplesCount, stats.TotalSamplesCount)
	assert.Equal(t, provider.stats.Type, stats.Type)
	assert.Equal(t, provider.stats.ID, stats.ID)
	assert.Equal(t, provider.stats.Kind, stats.Kind)
	assert.Equal(t, statsTimestampFrom(now), stats.Timestamp)
	assert.Equal(t, 1, provider.calls)
	assert.Equal(t, now, provider.lastNow)
}

func TestTrackRemotePullAudioPlayoutStatsMissingProvider(t *testing.T) {
	receiver := &RTPReceiver{}
	track := newTrackRemote(RTPCodecTypeAudio, 1111, 0, "", receiver)

	stats := track.pullAudioPlayoutStats(time.Now())
	require.Empty(t, stats)
}

func TestTrackRemotePullAudioPlayoutStatsProviderFalse(t *testing.T) {
	receiver := &RTPReceiver{}
	track := newTrackRemote(RTPCodecTypeAudio, 1111, 0, "", receiver)

	provider := &fakeTrackAudioPlayoutStatsProvider{ok: false}
	err := provider.AddTrack(track)
	require.NoError(t, err)

	stats := track.pullAudioPlayoutStats(time.Now())
	require.Empty(t, stats)
	assert.Equal(t, 1, provider.calls)
}

func TestTrackRemotePullAudioPlayoutStatsNormalizesDefaults(t *testing.T) {
	receiver := &RTPReceiver{}
	track := newTrackRemote(RTPCodecTypeAudio, 2468, 0, "", receiver)

	provider := &fakeTrackAudioPlayoutStatsProvider{
		stats: AudioPlayoutStats{
			TotalSamplesCount: 480,
		},
		ok: true,
	}

	err := provider.AddTrack(track)
	require.NoError(t, err)

	allStats := track.pullAudioPlayoutStats(time.Unix(10, 0))
	require.Len(t, allStats, 1)
	stats := allStats[0]

	assert.Equal(t, "media-playout-2468", stats.ID)
	assert.Equal(t, StatsTypeMediaPlayout, stats.Type)
	assert.Equal(t, string(MediaKindAudio), stats.Kind)
	assert.NotZero(t, stats.Timestamp)
}
