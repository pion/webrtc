// Package samplebuilder provides functionality to reconstruct media frame from RTP packets
package samplebuilder

import (
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v2/pkg/media"
)

// SampleBuilder contains all packets
// maxLate determines how long we should wait until we get a valid Sample
// The larger the value the less packet loss you will see, but higher latency
type SampleBuilder struct {
	maxLate uint16
	buffer  [65536]*rtp.Packet

	// Interface that allows us to take RTP packets to samples
	depacketizer rtp.Depacketizer

	// Last seqnum that has been added to buffer
	lastPush uint16

	// Last seqnum that has been successfully popped
	// isContiguous is false when we start or when we have a gap
	// that is older then maxLate
	isContiguous     bool
	lastPopSeq       uint16
	lastPopTimestamp uint32

	// Interface that checks whether the packet is the first fragment of the frame or not
	partitionHeadChecker rtp.PartitionHeadChecker
}

// New constructs a new SampleBuilder
func New(maxLate uint16, depacketizer rtp.Depacketizer, opts ...Option) *SampleBuilder {
	s := &SampleBuilder{maxLate: maxLate, depacketizer: depacketizer}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Push adds a RTP Packet to the sample builder
func (s *SampleBuilder) Push(p *rtp.Packet) {
	s.buffer[p.SequenceNumber] = p
	s.lastPush = p.SequenceNumber
	s.buffer[p.SequenceNumber-s.maxLate] = nil
}

// We have a valid collection of RTP Packets
// walk forwards building a sample if everything looks good clear and update buffer+values
func (s *SampleBuilder) buildSample(firstBuffer uint16) (*media.Sample, uint32) {
	data := []byte{}

	for i := firstBuffer; s.buffer[i] != nil; i++ {
		if s.buffer[i].Timestamp != s.buffer[firstBuffer].Timestamp {
			lastTimeStamp := s.lastPopTimestamp
			if !s.isContiguous {
				if s.buffer[firstBuffer-1] != nil {
					lastTimeStamp = s.buffer[firstBuffer-1].Timestamp
				} else {
					// If PartitionHeadChecker detects that the first packet is a head,
					// the duration of the packet is not guessable
					lastTimeStamp = s.buffer[firstBuffer].Timestamp
				}
			}

			samples := s.buffer[i-1].Timestamp - lastTimeStamp
			s.lastPopSeq = i - 1
			s.isContiguous = true
			s.lastPopTimestamp = s.buffer[i-1].Timestamp
			for j := firstBuffer; j < i; j++ {
				s.buffer[j] = nil
			}
			return &media.Sample{Data: data, Samples: samples}, s.lastPopTimestamp
		}

		p, err := s.depacketizer.Unmarshal(s.buffer[i].Payload)
		if err != nil {
			return nil, 0
		}

		data = append(data, p...)
	}
	return nil, 0
}

// Distance between two seqnums
func seqnumDistance(x, y uint16) uint16 {
	diff := int16(x - y)
	if diff < 0 {
		return uint16(-diff)
	}

	return uint16(diff)
}

// Pop scans buffer for valid samples, returns nil when no valid samples have been found
func (s *SampleBuilder) Pop() *media.Sample {
	sample, _ := s.PopWithTimestamp()
	return sample
}

// PopWithTimestamp scans buffer for valid samples and its RTP timestamp,
// returns nil, 0 when no valid samples have been found
func (s *SampleBuilder) PopWithTimestamp() (*media.Sample, uint32) {
	var i uint16
	if !s.isContiguous {
		i = s.lastPush - s.maxLate
	} else {
		if seqnumDistance(s.lastPopSeq, s.lastPush) > s.maxLate {
			i = s.lastPush - s.maxLate
			s.isContiguous = false
		} else {
			i = s.lastPopSeq + 1
		}
	}

	for ; i != s.lastPush; i++ {
		curr := s.buffer[i]
		if curr == nil {
			continue // we haven't hit a buffer yet, keep moving
		}

		if !s.isContiguous {
			if s.partitionHeadChecker == nil {
				if s.buffer[i-1] == nil {
					continue // We have never popped a buffer, so we can't assert that the first RTP packet we encounter is valid
				} else if s.buffer[i-1].Timestamp == curr.Timestamp {
					continue // We have the same timestamps, so it is data that spans multiple RTP packets
				}
			} else {
				if !s.partitionHeadChecker.IsPartitionHead(curr.Payload) {
					continue
				}
				// We can start using this frame as it is a head of frame partition
			}
		}

		// Initial validity checks have passed, walk forward
		return s.buildSample(i)
	}
	return nil, 0
}

// Option configures SampleBuilder
type Option func(o *SampleBuilder)

// WithPartitionHeadChecker assigns codec specific PartitionHeadChecker to SampleBuilder
func WithPartitionHeadChecker(checker rtp.PartitionHeadChecker) Option {
	return func(o *SampleBuilder) {
		o.partitionHeadChecker = checker
	}
}
