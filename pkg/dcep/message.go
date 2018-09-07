package dcep

import (
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
