// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"sync"
	"time"
)

// GetConnectionStats is a helper method to return the associated stats for a given PeerConnection.
func (r StatsReport) GetConnectionStats(conn *PeerConnection) (PeerConnectionStats, bool) {
	statsID := conn.ID()
	stats, ok := r[statsID]
	if !ok {
		return PeerConnectionStats{}, false
	}

	pcStats, ok := stats.(PeerConnectionStats)
	if !ok {
		return PeerConnectionStats{}, false
	}

	return pcStats, true
}

// GetDataChannelStats is a helper method to return the associated stats for a given DataChannel.
func (r StatsReport) GetDataChannelStats(dc *DataChannel) (DataChannelStats, bool) {
	statsID := dc.getStatsID()
	stats, ok := r[statsID]
	if !ok {
		return DataChannelStats{}, false
	}

	dcStats, ok := stats.(DataChannelStats)
	if !ok {
		return DataChannelStats{}, false
	}

	return dcStats, true
}

// GetICECandidateStats is a helper method to return the associated stats for a given ICECandidate.
func (r StatsReport) GetICECandidateStats(c *ICECandidate) (ICECandidateStats, bool) {
	statsID := c.statsID
	stats, ok := r[statsID]
	if !ok {
		return ICECandidateStats{}, false
	}

	candidateStats, ok := stats.(ICECandidateStats)
	if !ok {
		return ICECandidateStats{}, false
	}

	return candidateStats, true
}

// GetICECandidatePairStats is a helper method to return the associated stats for a given ICECandidatePair.
func (r StatsReport) GetICECandidatePairStats(c *ICECandidatePair) (ICECandidatePairStats, bool) {
	statsID := c.statsID
	stats, ok := r[statsID]
	if !ok {
		return ICECandidatePairStats{}, false
	}

	candidateStats, ok := stats.(ICECandidatePairStats)
	if !ok {
		return ICECandidatePairStats{}, false
	}

	return candidateStats, true
}

// GetCertificateStats is a helper method to return the associated stats for a given Certificate.
func (r StatsReport) GetCertificateStats(c *Certificate) (CertificateStats, bool) {
	statsID := c.statsID
	stats, ok := r[statsID]
	if !ok {
		return CertificateStats{}, false
	}

	certificateStats, ok := stats.(CertificateStats)
	if !ok {
		return CertificateStats{}, false
	}

	return certificateStats, true
}

// GetCodecStats is a helper method to return the associated stats for a given Codec.
func (r StatsReport) GetCodecStats(c *RTPCodecParameters) (CodecStats, bool) {
	statsID := c.statsID
	stats, ok := r[statsID]
	if !ok {
		return CodecStats{}, false
	}

	codecStats, ok := stats.(CodecStats)
	if !ok {
		return CodecStats{}, false
	}

	return codecStats, true
}

// AudioPlayoutStatsProvider is an interface for getting audio playout metrics.
type AudioPlayoutStatsProvider interface {
	// AddTrack registers a track to report playout stats to this provider.
	AddTrack(track *TrackRemote) error

	// RemoveTrack unregisters a track from this provider.
	RemoveTrack(track *TrackRemote)

	// Snapshot returns the accumulated stats at the given time.
	Snapshot(now time.Time) (AudioPlayoutStats, bool)
}

type trackContext struct {
	cancel context.CancelFunc
}

// defaultAudioPlayoutStatsProvider accumulates audio playout stats on behalf of the application.
type defaultAudioPlayoutStatsProvider struct {
	mu sync.Mutex

	stats           AudioPlayoutStats
	lastSynthesized bool
	tracks          map[*TrackRemote]*trackContext
}

// NewAudioPlayoutStatsProvider constructs a default provider with the supplied stats ID.
func NewAudioPlayoutStatsProvider(id string) *defaultAudioPlayoutStatsProvider {
	return &defaultAudioPlayoutStatsProvider{
		stats: AudioPlayoutStats{
			ID:   id,
			Type: StatsTypeMediaPlayout,
			Kind: string(MediaKindAudio),
		},
		tracks: make(map[*TrackRemote]*trackContext),
	}
}

// Accumulate applies a new batch of played-out samples to the running totals.
func (p *defaultAudioPlayoutStatsProvider) Accumulate(
	samples int, sampleRate uint32, deviceDelay time.Duration, synthesized bool,
) {
	if samples <= 0 || sampleRate == 0 {
		return
	}

	delaySeconds := deviceDelay.Seconds()
	if delaySeconds < 0 {
		delaySeconds = 0
	}

	duration := float64(samples) / float64(sampleRate)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.TotalSamplesCount += uint64(samples)
	p.stats.TotalSamplesDuration += duration
	p.stats.TotalPlayoutDelay += delaySeconds * float64(samples)

	if synthesized {
		p.stats.SynthesizedSamplesDuration += duration
		if !p.lastSynthesized {
			p.stats.SynthesizedSamplesEvents++
		}
	}

	p.lastSynthesized = synthesized
}

// Snapshot returns the accumulated stats at the given time.
func (p *defaultAudioPlayoutStatsProvider) Snapshot(now time.Time) (AudioPlayoutStats, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stats.TotalSamplesCount == 0 {
		return AudioPlayoutStats{}, false
	}

	stats := p.stats
	stats.Timestamp = statsTimestampFrom(now)

	return stats, true
}

// AddTrack registers a track to report playout stats to this provider.
func (p *defaultAudioPlayoutStatsProvider) AddTrack(track *TrackRemote) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.tracks[track]; exists {
		return nil
	}

	track.addProvider(p)

	ctx, cancel := context.WithCancel(context.Background())
	p.tracks[track] = &trackContext{cancel: cancel}

	go func() {
		receiver := track.receiver
		if receiver == nil {
			cancel()

			return
		}

		select {
		case <-receiver.closedChan:
			p.removeTrackInternal(track)
		case <-ctx.Done():
			return
		}
	}()

	return nil
}

// RemoveTrack unregisters a track from this provider.
func (p *defaultAudioPlayoutStatsProvider) RemoveTrack(track *TrackRemote) {
	p.removeTrackInternal(track)
}

func (p *defaultAudioPlayoutStatsProvider) removeTrackInternal(track *TrackRemote) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if tc, exists := p.tracks[track]; exists {
		tc.cancel()
		delete(p.tracks, track)
	}

	track.removeProvider(p)
}
