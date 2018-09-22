package samplebuilder

import (
	"github.com/pions/webrtc/pkg/media"
	"github.com/pions/webrtc/pkg/rtp"
)

// SampleBuilder contains all packets
// maxLate determines how long we should wait until we get a valid RTCSample
// The larger the value the less packet loss you will see, but higher latency
type SampleBuilder struct {
	maxLate uint16
	buffer  [65536]*rtp.Packet

	// Interface that allows us to take RTP packets to samples
	depacketizer rtp.Depacketizer

	// Last seqnum that has been added to buffer
	lastPush uint16

	// Last seqnum that has been successfully popped
	hasPopped        bool
	lastPopSeq       uint16
	lastPopTimestamp uint32
}

// New constructs a new SampleBuilder
func New(maxLate uint16, depacketizer rtp.Depacketizer) *SampleBuilder {
	return &SampleBuilder{maxLate: maxLate, depacketizer: depacketizer}
}

// Push adds a RTP Packet to the sample builder
func (s *SampleBuilder) Push(p *rtp.Packet) {
	s.buffer[p.SequenceNumber] = p
	s.lastPush = p.SequenceNumber
	s.buffer[p.SequenceNumber-s.maxLate] = nil

}

// We have a valid collection of RTP Packets
// walk forwards building a sample if everything looks good clear and update buffer+values
func (s *SampleBuilder) buildSample(firstBuffer uint16) *media.RTCSample {
	data := []byte{}

	for i := firstBuffer; s.buffer[i] != nil; i++ {
		if s.buffer[i].Timestamp != s.buffer[firstBuffer].Timestamp {
			lastTimeStamp := s.lastPopTimestamp
			if !s.hasPopped && s.buffer[firstBuffer-1] != nil {
				// firstBuffer-1 should always pass, but just to be safe if there is a bug in Pop()
				lastTimeStamp = s.buffer[firstBuffer-1].Timestamp
			}

			samples := s.buffer[i-1].Timestamp - lastTimeStamp
			s.lastPopSeq = i - 1
			s.hasPopped = true
			s.lastPopTimestamp = s.buffer[i-1].Timestamp
			for j := firstBuffer; j < i; j++ {
				s.buffer[j] = nil
			}
			return &media.RTCSample{Data: data, Samples: samples}
		}

		p, err := s.depacketizer.Unmarshal(s.buffer[i])
		if err != nil {
			return nil
		}

		data = append(data, p...)
	}
	return nil
}

// Pop scans buffer for valid samples, returns nil when no valid samples have been found
func (s *SampleBuilder) Pop() *media.RTCSample {
	var i uint16
	if !s.hasPopped {
		i = s.lastPush - s.maxLate
	} else {
		i = s.lastPopSeq + 1
	}

	for ; i != s.lastPush; i++ {
		curr := s.buffer[i]
		if curr == nil {
			if s.buffer[i-1] != nil {
				break // there is a gap, we can't proceed
			}

			continue // we haven't hit a buffer yet, keep moving
		}

		if !s.hasPopped {
			if s.buffer[i-1] == nil {
				continue // We have never popped a buffer, so we can't assert that the first RTP packet we encounter is valid
			} else if s.buffer[i-1].Timestamp == curr.Timestamp {
				continue // We have the same timestamps, so it is data that spans multiple RTP packets
			}
		}

		// Initial validity checks have passed, walk forward
		return s.buildSample(i)
	}
	return nil
}
