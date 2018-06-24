package rtp

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

type Sequencer interface {
	NextSequenceNumber() uint16
}

func NewRandomSequencer() Sequencer {
	rs := rand.NewSource(time.Now().UnixNano())
	r := rand.New(rs)

	return &sequencer{
		sequenceNumber: uint16(r.Uint32() % math.MaxUint16),
	}
}

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

func (s *sequencer) NextSequenceNumber() uint16 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.sequenceNumber++
	if s.sequenceNumber == 0 {
		s.rollOverCount++
	}

	return s.sequenceNumber
}
