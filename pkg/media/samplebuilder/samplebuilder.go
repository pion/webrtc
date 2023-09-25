// Package samplebuilder builds media frames from RTP packets.
package samplebuilder

import (
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4/pkg/media"
)

type packet struct {
	start, end bool
	packet     *rtp.Packet
}

// SampleBuilder buffers packets and produces media frames
type SampleBuilder struct {
	// a circular array of buffered packets.  We interpret head=tail
	// as an empty builder, so there's always a free slot.  The
	// invariants that this data structure obeys are codified in the
	// function check below.
	packets    []packet
	head, tail uint16

	maxLate              uint16
	depacketizer         rtp.Depacketizer
	packetReleaseHandler func(*rtp.Packet)
	sampleRate           uint32

	// indicates whether the lastSeqno field is valid
	lastSeqnoValid bool
	// the seqno of the last popped or dropped packet, if any
	lastSeqno uint16

	// indicates whether the lastTimestamp field is valid
	lastTimestampValid bool
	// the timestamp of the last popped packet, if any.
	lastTimestamp uint32
}

// New constructs a new SampleBuilder.
//
// maxLate is the maximum delay, in RTP sequence numbers, that the samplebuilder
// will wait before dropping a frame.  The actual buffer size is twice as large
// in order to compensate for delays between Push and Pop.
func New(maxLate uint16, depacketizer rtp.Depacketizer, sampleRate uint32, opts ...Option) *SampleBuilder {
	if maxLate < 2 {
		maxLate = 2
	}
	if maxLate > 0x7FFF {
		maxLate = 0x7FFF
	}
	s := &SampleBuilder{
		packets:      make([]packet, 2*maxLate+1),
		maxLate:      maxLate,
		depacketizer: depacketizer,
		sampleRate:   sampleRate,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// An Option configures a SampleBuilder
type Option func(o *SampleBuilder)

// WithPacketReleaseHandler sets a callback that is called when the
// builder is about to release some packet.
func WithPacketReleaseHandler(h func(*rtp.Packet)) Option {
	return func(s *SampleBuilder) {
		s.packetReleaseHandler = h
	}
}

// check verifies the samplebuilder's invariants.  It may be used in testing.
// nolint: gocognit
func (s *SampleBuilder) check() {
	if s.head == s.tail {
		return
	}

	// the entry at tail must not be missing
	if s.packets[s.tail].packet == nil {
		panic("tail is missing")
	}
	// the entry at head-1 must not be missing
	if s.packets[s.dec(s.head)].packet == nil {
		panic("head is missing")
	}
	if s.lastSeqnoValid {
		// the last dropped packet is before tail
		diff := s.packets[s.tail].packet.SequenceNumber - s.lastSeqno
		if diff == 0 || diff&0x8000 != 0 {
			panic("lastSeqno is after tail")
		}
	}

	// indices are sequential, and the start and end flags are correct
	tailSeqno := s.packets[s.tail].packet.SequenceNumber
	for i := uint16(0); i < uint16(s.Len()); i++ {
		index := (s.tail + i) % uint16(len(s.packets))
		if s.packets[index].packet == nil {
			continue
		}
		if s.packets[index].packet.SequenceNumber != tailSeqno+i {
			panic("wrong seqno")
		}
		ts := s.packets[index].packet.Timestamp
		if index != s.tail && !s.packets[index].start {
			prev := s.dec(index)
			if s.packets[prev].packet != nil && s.packets[prev].packet.Timestamp != ts {
				panic("start is not set")
			}
		}
		if index != s.dec(s.head) && !s.packets[index].end {
			next := s.inc(index)
			if s.packets[next].packet != nil && s.packets[next].packet.Timestamp != ts {
				panic("end is not set")
			}
		}
	}
	// all packets outside of the interval are missing
	for i := s.head; i != s.tail; i = s.inc(i) {
		if s.packets[i].packet != nil {
			panic("packet is set")
		}
	}
}

// Len returns the difference minus one between the smallest and the
// largest sequence number stored in the SampleBuilder.
func (s *SampleBuilder) Len() int {
	if s.tail <= s.head {
		return int(s.head - s.tail)
	}
	return int(s.head + uint16(len(s.packets)) - s.tail)
}

// cap returns the capacity of the SampleBuilder.
func (s *SampleBuilder) cap() uint16 {
	// since head==tail indicates an empty builder, we always keep one
	// empty element
	return uint16(len(s.packets)) - 1
}

// inc adds one to an index.
func (s *SampleBuilder) inc(n uint16) uint16 {
	if n < uint16(len(s.packets))-1 {
		return n + 1
	}
	return 0
}

// dec subtracts one from an index.
func (s *SampleBuilder) dec(n uint16) uint16 {
	if n > 0 {
		return n - 1
	}
	return uint16(len(s.packets)) - 1
}

// isStart invokes the PartitionHeadChecker associted with s.
func (s *SampleBuilder) isStart(p *rtp.Packet) bool {
	return s.depacketizer.IsPartitionHead(p.Payload)
}

// isEnd invokes the partitionTailChecker associated with s.
func (s *SampleBuilder) isEnd(p *rtp.Packet) bool {
	return s.depacketizer.IsPartitionTail(p.Marker, p.Payload)
}

// release releases the last packet.
func (s *SampleBuilder) release() bool {
	if s.head == s.tail {
		return false
	}
	s.lastSeqnoValid = true
	s.lastSeqno = s.packets[s.tail].packet.SequenceNumber
	if s.packetReleaseHandler != nil {
		s.packetReleaseHandler(s.packets[s.tail].packet)
	}
	s.packets[s.tail] = packet{}
	s.tail = s.inc(s.tail)
	for s.tail != s.head && s.packets[s.tail].packet == nil {
		s.tail = s.inc(s.tail)
	}
	if s.tail == s.head {
		s.head = 0
		s.tail = 0
	}
	return true
}

// releaseAll releases all packets.
func (s *SampleBuilder) releaseAll() {
	for s.tail != s.head {
		s.release()
	}
}

// drop drops the last frame, even if it is incomplete.  It returns true
// if a packet has been dropped, and the dropped packet's timestamp.
func (s *SampleBuilder) drop() (bool, uint32) {
	if s.tail == s.head {
		return false, 0
	}
	ts := s.packets[s.tail].packet.Timestamp
	s.release()
	for s.tail != s.head {
		if s.packets[s.tail].start ||
			s.packets[s.tail].packet.Timestamp != ts {
			break
		}
		s.release()
	}
	if !s.lastTimestampValid {
		s.lastTimestamp = ts
		s.lastTimestampValid = true
	}
	return true, ts
}

// Push adds an RTP Packet to s's buffer.
//
// Push does not copy the input: the packet will be retained by s.  If you
// plan to reuse the packet or its buffer, make sure to perform a copy.
// nolint: gocognit
func (s *SampleBuilder) Push(p *rtp.Packet) {
	if s.lastSeqnoValid {
		if (s.lastSeqno-p.SequenceNumber)&0x8000 == 0 {
			// late packet
			if s.lastSeqno-p.SequenceNumber > s.maxLate {
				s.lastSeqnoValid = false
			} else {
				return
			}
		} else {
			last := p.SequenceNumber - s.maxLate
			if (last-s.lastSeqno)&0x8000 == 0 {
				if s.head != s.tail {
					seqno := s.packets[s.tail].packet.SequenceNumber - 1
					if (last-seqno)&0x8000 == 0 {
						last = seqno
					}
				}
				s.lastSeqno = last
			}
		}
	}

	if s.head == s.tail {
		// empty
		s.packets[0] = packet{
			start:  s.isStart(p),
			end:    s.isEnd(p),
			packet: p,
		}
		s.tail = 0
		s.head = 1
		return
	}

	seqno := p.SequenceNumber
	ts := p.Timestamp
	last := s.dec(s.head)
	lastSeqno := s.packets[last].packet.SequenceNumber
	if seqno == lastSeqno+1 {
		// sequential
		if s.tail == s.inc(s.head) {
			s.drop()
		}
		start := false
		// drop may have dropped the whole buffer
		if s.tail != s.head {
			start = s.packets[last].end ||
				s.packets[last].packet.Timestamp != p.Timestamp ||
				s.isStart(p)
			if start {
				s.packets[last].end = true
			}
		} else {
			start = s.isStart(p)
		}
		s.packets[s.head] = packet{
			start:  start,
			end:    s.isEnd(p),
			packet: p,
		}
		s.head = s.inc(s.head)
		return
	}

	if ((seqno - lastSeqno) & 0x8000) == 0 {
		// packet in the future
		count := seqno - lastSeqno - 1
		if count >= s.cap() {
			s.releaseAll()
			s.Push(p)
			return
		}
		// make free space
		for uint16(s.Len())+count+1 >= s.cap() {
			dropped, _ := s.drop()
			if !dropped {
				// this shouldn't happen
				return
			}
		}
		index := (s.head + count) % uint16(len(s.packets))
		start := s.isStart(p)
		s.packets[index] = packet{
			start:  start,
			end:    s.isEnd(p),
			packet: p,
		}
		s.head = s.inc(index)
		return
	}

	// packet is in the past
	count := lastSeqno - seqno + 1
	if count >= s.cap() {
		// too old
		return
	}

	var index uint16
	if s.head >= count {
		index = s.head - count
	} else {
		index = s.head + uint16(len(s.packets)) - count
	}

	// extend if necessary
	if s.tail < s.head {
		// buffer is contigous
		if index < s.tail || index > s.head {
			s.tail = index
		}
	} else {
		// buffer is discontigous
		if index < s.tail && index > s.head {
			s.tail = index
		}
	}

	if s.packets[index].packet != nil {
		// duplicate packet
		if s.packetReleaseHandler != nil {
			s.packetReleaseHandler(p)
		}
		return
	}

	// compute start and end flags, both for us and our neighbors
	start := s.isStart(p)
	if index != s.tail {
		prev := s.dec(index)
		if s.packets[prev].packet != nil {
			if s.packets[prev].packet.Timestamp != ts {
				start = true
			}
			if !start {
				start = s.packets[prev].end
			} else {
				s.packets[prev].end = true
			}
		}
	}
	end := s.isEnd(p)
	next := s.inc(index)
	if s.packets[next].packet != nil {
		if s.packets[next].packet.Timestamp != ts {
			end = true
		}
		if !end {
			end = s.packets[next].start
		} else {
			s.packets[next].start = true
		}
	}

	// done!
	s.packets[index] = packet{
		start:  start,
		end:    end,
		packet: p,
	}
}

func (s *SampleBuilder) pop(force bool) (*media.Sample, uint32) {
again:
	if s.tail == s.head {
		return nil, 0
	}

	if !s.packets[s.tail].start {
		diff := s.packets[s.dec(s.head)].packet.SequenceNumber -
			s.packets[s.tail].packet.SequenceNumber
		if force || diff > s.maxLate {
			s.drop()
			goto again
		}
		return nil, 0
	}

	seqno := s.packets[s.tail].packet.SequenceNumber
	if !force && s.lastSeqnoValid && s.lastSeqno+1 != seqno {
		// packet loss before tail
		return nil, 0
	}

	ts := s.packets[s.tail].packet.Timestamp
	last := s.tail
	for last != s.head && !s.packets[last].end {
		if s.packets[last].packet == nil {
			if force {
				s.drop()
				goto again
			}
			return nil, 0
		}
		last = s.inc(last)
	}

	if last == s.head {
		return nil, 0
	}

	var data []byte
	count := last - s.tail + 1
	if last < s.tail {
		count = s.cap() + last - s.tail + 1
	}
	for i := uint16(0); i < count; i++ {
		buf, err := s.depacketizer.Unmarshal(
			s.packets[s.tail].packet.Payload,
		)
		s.release()
		if err != nil {
			return nil, 0
		}
		data = append(data, buf...)
	}

	var samples uint32
	if s.lastTimestampValid {
		samples = ts - s.lastTimestamp
	}

	s.lastTimestampValid = true
	s.lastTimestamp = ts
	duration := time.Duration(float64(samples) / float64(s.sampleRate) * float64(time.Second))

	return &media.Sample{
		Data:     data,
		Duration: duration,
	}, ts
}

// PopWithTimestamp returns a completed packet and its RTP timestamp.  If
// the oldest packet is incomplete and hasn't reached MaxLate yet, Pop
// returns nil.
func (s *SampleBuilder) PopWithTimestamp() (*media.Sample, uint32) {
	return s.pop(false)
}

// Pop returns a completed packet.  If the oldest packet is incomplete and
// hasn't reached MaxLate yet, Pop returns nil.
func (s *SampleBuilder) Pop() *media.Sample {
	sample, _ := s.PopWithTimestamp()
	return sample
}

// ForcePopWithTimestamp is like PopWithTimestamp, but will always pops
// a sample if any are available, even if it's being blocked by a missing
// packet.  This is useful when the stream ends, or after a link outage.
// After ForcePopWithTimestamp returns nil, the samplebuilder is
// guaranteed to be empty.
func (s *SampleBuilder) ForcePopWithTimestamp() (*media.Sample, uint32) {
	return s.pop(true)
}
