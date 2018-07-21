package datachannel

import (
	"github.com/pkg/errors"
)

type ChannelAck struct{}

const (
	channelOpenAckLength = 4
)

// Marshal returns raw bytes for the given message
func (c *ChannelAck) Marshal() ([]byte, error) {
	raw := make([]byte, channelOpenAckLength)
	raw[0] = uint8(DataChannelAck)

	return raw, nil
}

// Unmarshal populates the struct with the given raw data
func (c *ChannelAck) Unmarshal(raw []byte) error {
	return errors.Errorf("Unimplemented")
}
