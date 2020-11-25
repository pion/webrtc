package interceptor

import (
	"errors"
	"strconv"

	"github.com/pion/rtp"
)

const (
	uint16SizeHalf = 1 << 15
)

var (
	allowedSendBufferSizes     map[uint16]bool
	invalidSendBufferSizeError string
)

func init() {
	allowedSendBufferSizes = make(map[uint16]bool, 15)
	invalidSendBufferSizeError = "invalid sendBuffer size, must be one of: "
	for i := 0; i < 16; i++ {
		allowedSendBufferSizes[1<<i] = true
		invalidSendBufferSizeError += strconv.Itoa(1<<i) + ", "
	}
	invalidSendBufferSizeError = invalidSendBufferSizeError[0 : len(invalidSendBufferSizeError)-2]
}

type SendBuffer struct {
	packets   []*rtp.Packet
	size      uint16
	lastAdded uint16
	started   bool
}

func NewSendBuffer(size uint16) (*SendBuffer, error) {
	if !allowedSendBufferSizes[size] {
		return nil, errors.New(invalidSendBufferSizeError)
	}

	return &SendBuffer{
		packets: make([]*rtp.Packet, size),
		size:    size,
	}, nil
}

func (s *SendBuffer) Add(packet *rtp.Packet) {
	seq := packet.SequenceNumber
	if !s.started {
		s.packets[seq%s.size] = packet
		s.lastAdded = seq
		s.started = true
		return
	}

	diff := seq - s.lastAdded
	if diff == 0 {
		return
	} else if diff < uint16SizeHalf {
		for i := s.lastAdded + 1; i != seq; i++ {
			s.packets[i%s.size] = nil
		}
	}

	s.packets[seq%s.size] = packet
	s.lastAdded = seq
}

func (s *SendBuffer) Get(seq uint16) *rtp.Packet {
	diff := s.lastAdded - seq
	if diff >= uint16SizeHalf {
		return nil
	}

	if diff >= s.size {
		return nil
	}

	return s.packets[seq%s.size]
}
