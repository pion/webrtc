package samplebuilder

import (
	"fmt"

	"github.com/pions/webrtc/pkg/media"
	"github.com/pions/webrtc/pkg/rtp"
)

// SampleBuilder contains all packets
// maxLate determines how long we should wait until we get a valid RTCSample
// The larger the value the less packet loss you will see, but higher latency
type SampleBuilder struct {
	maxLate  uint16
	lastPush uint16

	buffer [65536]*rtp.Packet
}

// New constructs a new SampleBuilder
func New(maxLate uint16) *SampleBuilder {
	return &SampleBuilder{maxLate: maxLate}
}

// Push adds a RTP Packet to the sample builder
func (s *SampleBuilder) Push(p *rtp.Packet) {
	s.buffer[p.SequenceNumber] = p
	s.lastPush = p.SequenceNumber
	s.buffer[p.SequenceNumber-s.maxLate] = nil

}

// Pop scans our buffer for valid samples, returns nil when no valid samples have been found
func (s *SampleBuilder) Pop() *media.RTCSample {
	for i := s.lastPush - s.maxLate; i != s.lastPush; i++ {
		if curr := s.buffer[i]; curr != nil {
			fmt.Println(curr)
		}
	}

	return nil
}
