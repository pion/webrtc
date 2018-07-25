package datachannel

import (
	"testing"

	"github.com/pkg/errors"
)

func TestChannelOpenUnmarshal(t *testing.T) {
	rawMsg := []byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x66, 0x6f, 0x6f}
	msgUncast, err := Parse(rawMsg)

	msg, ok := msgUncast.(*ChannelOpen)
	if !ok {
		t.Error(errors.Errorf("Failed to cast to ChannelOpen"))
	}

	if err != nil {
		t.Error(errors.Wrap(err, "Unmarshal failed, ChannelOpen"))
	} else if msg.ChannelType != 0 {
		t.Error(errors.Errorf("ChannelType should be 0"))
	} else if msg.Priority != 0 {
		t.Error(errors.Errorf("Priority should be 0"))
	} else if msg.ReliabilityParameter != 0 {
		t.Error(errors.Errorf("ReliabilityParameter should be 0"))
	} else if string(msg.Label) != "foo" {
		t.Error(errors.Errorf("msg Label should be 'foo'"))
	}
}
