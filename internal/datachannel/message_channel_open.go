package datachannel

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

/*
ChannelOpen represents a DATA_CHANNEL_OPEN Message

 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Message Type |  Channel Type |            Priority           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Reliability Parameter                      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|         Label Length          |       Protocol Length         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|                             Label                             |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|                            Protocol                           |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/
type ChannelOpen struct {
	ChannelType          byte
	Priority             uint16
	ReliabilityParameter uint32

	Label    []byte
	Protocol []byte
}

const (
	channelOpenHeaderLength = 12
)

// Marshal returns raw bytes for the given message
func (c *ChannelOpen) Marshal() ([]byte, error) {
	return nil, errors.Errorf("Unimplemented")
}

// Unmarshal populates the struct with the given raw data
func (c *ChannelOpen) Unmarshal(raw []byte) error {
	if len(raw) < channelOpenHeaderLength {
		return errors.Errorf("Length of input is not long enough to satisfy header %d", len(raw))
	}
	c.ChannelType = raw[1]
	c.Priority = binary.BigEndian.Uint16(raw[2:])
	c.ReliabilityParameter = binary.BigEndian.Uint32(raw[4:])

	labelLength := binary.BigEndian.Uint16(raw[8:])
	protocolLength := binary.BigEndian.Uint16(raw[10:])

	if len(raw) != int(channelOpenHeaderLength+labelLength+protocolLength) {
		return errors.Errorf("Label + Protocol length don't match full packet length")
	}

	c.Label = raw[12 : 12+labelLength]
	c.Protocol = raw[12+labelLength : 12+labelLength+protocolLength]
	return nil
}
