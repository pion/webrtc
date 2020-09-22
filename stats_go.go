// +build !js

package webrtc

// GetConnectionStats is a helper method to return the associated stats for a given PeerConnection
func (r StatsReport) GetConnectionStats(conn *PeerConnection) (PeerConnectionStats, bool) {
	statsID := conn.getStatsID()
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

// GetDataChannelStats is a helper method to return the associated stats for a given DataChannel
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

// GetICECandidateStats is a helper method to return the associated stats for a given ICECandidate
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

// GetICECandidatePairStats is a helper method to return the associated stats for a given ICECandidatePair
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

// GetCertificateStats is a helper method to return the associated stats for a given Certificate
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

// GetCodecStats is a helper method to return the associated stats for a given Codec
func (r StatsReport) GetCodecStats(c *RTPCodec) (CodecStats, bool) {
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

// GetAudioSenderStats is a helper method to return the associated stats for a given AudioSender
func (r StatsReport) GetAudioSenderStats(s *RTPSender) (AudioSenderStats, bool) {
	statsID := s.statsID
	stats, ok := r[statsID]
	if !ok {
		return AudioSenderStats{}, false
	}

	audioSenderStats, ok := stats.(AudioSenderStats)
	if !ok {
		return AudioSenderStats{}, false
	}
	return audioSenderStats, true
}

// GetVideoSenderStats is a helper method to return the associated stats for a given AudioSender
func (r StatsReport) GetVideoSenderStats(s *RTPSender) (VideoSenderStats, bool) {
	statsID := s.statsID
	stats, ok := r[statsID]
	if !ok {
		return VideoSenderStats{}, false
	}

	videoSenderStats, ok := stats.(VideoSenderStats)
	if !ok {
		return VideoSenderStats{}, false
	}
	return videoSenderStats, true
}

// GetSenderStats is a helper method to return the associated stats for a given Sender
func (r StatsReport) GetSenderStats(s *RTPSender) (GetStatsType, bool) {
	if s.Track().Kind() == RTPCodecTypeAudio {
		return r.GetAudioSenderStats(s)
	} else {
		return r.GetVideoSenderStats(s)
	}
}

// GetSenderStats is a helper method to return the associated stats for a given Receiver
func (r StatsReport) GetReceiverStats(s *RTPReceiver) (GetStatsType, bool) {
	if s.Track().Kind() == RTPCodecTypeAudio {
		return r.GetAudioReceiverStats(s)
	} else {
		return r.GetVideoReceiverStats(s)
	}
}

func (a AudioReceiverStats) getType() StatsType {
	return a.Type
}

func (v VideoReceiverStats) getType() StatsType {
	return v.Type
}

func (a AudioSenderStats) getType() StatsType {
	return a.Type
}

func (v VideoSenderStats) getType() StatsType {
	return v.Type
}

type GetStatsType interface {
	getType() StatsType
}

// GetAudioReceiverStats is a helper method to return the associated stats for a given AudioReceiver
func (r StatsReport) GetAudioReceiverStats(receiver *RTPReceiver) (AudioReceiverStats, bool) {
	statsID := receiver.statsID
	stats, ok := r[statsID]
	if !ok {
		return AudioReceiverStats{}, false
	}

	audioReceiverStats, ok := stats.(AudioReceiverStats)
	if !ok {
		return AudioReceiverStats{}, false
	}
	return audioReceiverStats, true
}

// GetVideoReceiverStats is a helper method to return the associated stats for a given VideoReceiver
func (r StatsReport) GetVideoReceiverStats(receiver *RTPReceiver) (VideoReceiverStats, bool) {
	statsID := receiver.statsID
	stats, ok := r[statsID]
	if !ok {
		return VideoReceiverStats{}, false
	}

	videoReceiverStats, ok := stats.(VideoReceiverStats)
	if !ok {
		return VideoReceiverStats{}, false
	}
	return videoReceiverStats, true
}
