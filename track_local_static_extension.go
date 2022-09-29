//go:build !js
// +build !js

package webrtc

import (
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/internal/util"
	"github.com/pion/webrtc/v3/pkg/media"
)

func (s *TrackLocalStaticSample) WriteSimulcastSample(sample media.Sample, extensions []rtp.Extension) error {

	s.rtpTrack.mu.RLock()
	p := s.packetizer
	clockRate := s.clockRate
	s.rtpTrack.mu.RUnlock()

	if p == nil {
		return nil
	}

	// skip packets by the number of previously dropped packets
	for i := uint16(0); i < sample.PrevDroppedPackets; i++ {
		s.sequencer.NextSequenceNumber()
	}

	samples := uint32(sample.Duration.Seconds() * clockRate)
	if sample.PrevDroppedPackets > 0 {
		p.SkipSamples(samples * uint32(sample.PrevDroppedPackets))
	}
	packets := p.Packetize(sample.Data, samples)

	writeErrs := []error{}
	for _, p := range packets {
		p.Header.Extension = true
		p.Header.ExtensionProfile = 0x1000
		p.Header.Extensions = extensions
		if err := s.rtpTrack.WriteRTP(p); err != nil {
			writeErrs = append(writeErrs, err)
		}
	}

	return util.FlattenErrs(writeErrs)
}
