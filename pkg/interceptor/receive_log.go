package interceptor

import (
	"errors"
	"strconv"
)

var (
	allowedReceiveLogSizes     map[uint16]bool
	invalidReceiveLogSizeError string
)

func init() {
	allowedReceiveLogSizes = make(map[uint16]bool, 15)
	invalidReceiveLogSizeError = "invalid ReceiveLog size, must be one of: "
	for i := 6; i < 16; i++ {
		allowedReceiveLogSizes[1<<i] = true
		invalidReceiveLogSizeError += strconv.Itoa(1<<i) + ", "
	}
	invalidReceiveLogSizeError = invalidReceiveLogSizeError[0 : len(invalidReceiveLogSizeError)-2]
}

type ReceiveLog struct {
	packets         []uint64
	size            uint16
	end             uint16
	started         bool
	lastConsecutive uint16
}

func NewReceiveLog(size uint16) (*ReceiveLog, error) {
	if !allowedReceiveLogSizes[size] {
		return nil, errors.New(invalidReceiveLogSizeError)
	}

	return &ReceiveLog{
		packets: make([]uint64, size/64),
		size:    size,
	}, nil
}

func (s *ReceiveLog) Add(seq uint16) {
	if !s.started {
		s.set(seq)
		s.end = seq
		s.started = true
		s.lastConsecutive = seq
		return
	}

	diff := seq - s.end
	if diff == 0 {
		return
	} else if diff < uint16SizeHalf {
		// this means a positive diff, in other words seq > end (with counting for rollovers)
		for i := s.end + 1; i != seq; i++ {
			// clear packets between end and seq (these may contain packets from a "size" ago)
			s.del(i)
		}
		s.end = seq

		if s.lastConsecutive+1 == seq {
			s.lastConsecutive = seq
		} else if seq-s.lastConsecutive > s.size {
			s.lastConsecutive = seq - s.size
			s.fixLastConsecutive() // there might be valid packets at the beginning of the buffer now
		}
	} else {
		// negative diff, seq < end (with counting for rollovers)
		if s.lastConsecutive+1 == seq {
			s.lastConsecutive = seq
			s.fixLastConsecutive() // there might be other valid packets after seq
		}
	}

	s.set(seq)
}

func (s *ReceiveLog) Get(seq uint16) bool {
	diff := s.end - seq
	if diff >= uint16SizeHalf {
		return false
	}

	if diff >= s.size {
		return false
	}

	return s.get(seq)
}

func (s *ReceiveLog) MissingSeqNumbers(skipLastN uint16) []uint16 {
	until := s.end - skipLastN
	if until-s.lastConsecutive >= uint16SizeHalf {
		// until < s.lastConsecutive (counting for rollover)
		return nil
	}

	missingPacketSeqNums := make([]uint16, 0)
	for i := s.lastConsecutive + 1; i != until+1; i++ {
		if !s.get(i) {
			missingPacketSeqNums = append(missingPacketSeqNums, i)
		}
	}

	return missingPacketSeqNums
}

func (s *ReceiveLog) set(seq uint16) {
	pos := seq % s.size
	s.packets[pos/64] |= 1 << (pos % 64)
}

func (s *ReceiveLog) del(seq uint16) {
	pos := seq % s.size
	s.packets[pos/64] &^= 1 << (pos % 64)
}

func (s *ReceiveLog) get(seq uint16) bool {
	pos := seq % s.size
	return (s.packets[pos/64] & (1 << (pos % 64))) != 0
}

func (s *ReceiveLog) fixLastConsecutive() {
	i := s.lastConsecutive + 1
	for ; i != s.end+1 && s.get(i); i++ {
		// find all consecutive packets
	}
	s.lastConsecutive = i - 1
}
