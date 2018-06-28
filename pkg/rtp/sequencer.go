package rtp

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// Sequencer generates sequential sequence numbers for building RTP packets
type Sequencer interface {
	NextSequenceNumber() uint16
	RollOverCount() uint64
}

// NewRandomSequencer returns a new sequencer starting from a random sequence
// number
func NewRandomSequencer() Sequencer {
	rs := rand.NewSource(time.Now().UnixNano())
	r := rand.New(rs)

	return &sequencer{
		sequenceNumber: uint16(r.Uint32() % math.MaxUint16),
	}
}

// NewFixedSequencer returns a new sequencer starting from a specific
// sequence number
func NewFixedSequencer(s uint16) Sequencer {
	return &sequencer{
		sequenceNumber: s - 1, // -1 because the first sequence number prepends 1
	}
}

type sequencer struct {
	sequenceNumber uint16
	rollOverCount  uint64
	mutex          sync.Mutex
}

// NextSequenceNumber increment and returns a new sequence number for
// building RTP packets
func (s *sequencer) NextSequenceNumber() uint16 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.sequenceNumber++
	if s.sequenceNumber == 0 {
		s.rollOverCount++
	}

	return s.sequenceNumber
}

// RollOverCount returns the amount of times the 16bit sequence number
// has wrapped
func (s *sequencer) RollOverCount() uint64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.rollOverCount
}
