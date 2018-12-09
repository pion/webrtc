package datachannel

import (
	"fmt"

	"github.com/pkg/errors"
)

// Message is a parsed DataChannel message
type Message interface {
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
}

// MessageType is the first byte in a DataChannel message that specifies type
type MessageType byte

// DataChannel Message Types
const (
	DataChannelAck  MessageType = 0x02
	DataChannelOpen MessageType = 0x03
)

func (t MessageType) String() string {
	switch t {
	case DataChannelAck:
		return "DataChannelAck"
	case DataChannelOpen:
		return "DataChannelOpen"
	default:
		return fmt.Sprintf("Unknown MessageType: %d", t)
	}
}

// Parse accepts raw input and returns a DataChannel message
func Parse(raw []byte) (Message, error) {
	if len(raw) == 0 {
		return nil, errors.Errorf("DataChannel message is not long enough to determine type ")
	}

	var msg Message
	switch MessageType(raw[0]) {
	case DataChannelOpen:
		msg = &ChannelOpen{}
	case DataChannelAck:
		msg = &ChannelAck{}
	default:
		return nil, errors.Errorf("Unknown MessageType %v", MessageType(raw[0]))
	}

	if err := msg.Unmarshal(raw); err != nil {
		return nil, err
	}

	return msg, nil
}

// ParseExpectDataChannelOpen parses a DataChannelOpen message
// or throws an error
func ParseExpectDataChannelOpen(raw []byte) (*ChannelOpen, error) {
	if len(raw) == 0 {
		return nil, errors.Errorf("the DataChannel message is not long enough to determine type")
	}

	if actualTyp := MessageType(raw[0]); actualTyp != DataChannelOpen {
		return nil, errors.Errorf("expected DataChannelOpen but got %s", actualTyp)
	}

	msg := &ChannelOpen{}
	if err := msg.Unmarshal(raw); err != nil {
		return nil, err
	}

	return msg, nil
}

// ParseExpectDataChannelAck parses a DataChannelAck message
// or throws an error
func ParseExpectDataChannelAck(raw []byte) (*ChannelAck, error) {
	if len(raw) == 0 {
		return nil, errors.Errorf("the DataChannel message is not long enough to determine type")
	}

	if actualTyp := MessageType(raw[0]); actualTyp != DataChannelAck {
		return nil, errors.Errorf("expected DataChannelAck but got %s", actualTyp)
	}

	msg := &ChannelAck{}
	if err := msg.Unmarshal(raw); err != nil {
		return nil, err
	}

	return msg, nil
}
