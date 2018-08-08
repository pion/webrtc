package datachannel

import (
	"testing"

	"github.com/pkg/errors"
)

func TestChannelOpenMarshal(t *testing.T) {
	msg := ChannelOpen{
		ChannelType:          0,
		Priority:             0,
		ReliabilityParameter: 0,

		Label:    []byte("foo"),
		Protocol: []byte("bar"),
	}

	rawMsg, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	result := []byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x03, 0x66, 0x6f, 0x6f, 0x62, 0x61, 0x72}

	if len(rawMsg) != len(result) {
		t.Fatalf("%q != %q", rawMsg, result)
	}

	for i, v := range rawMsg {
		if v != result[i] {
			t.Errorf("%q != %q", rawMsg, result)
			break
		}
	}
}

func TestChannelAckMarshal(t *testing.T) {
	msg := ChannelAck{}
	rawMsg, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	result := []byte{0x02, 0x00, 0x00, 0x00}

	if len(rawMsg) != len(result) {
		t.Fatalf("%q != %q", rawMsg, result)
	}

	for i, v := range rawMsg {
		if v != result[i] {
			t.Errorf("%q != %q", rawMsg, result)
			break
		}
	}
}

func TestChannelOpenUnmarshal(t *testing.T) {
	rawMsg := []byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x03, 0x66, 0x6f, 0x6f, 0x62, 0x61, 0x72}
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
	} else if string(msg.Protocol) != "bar" {
		t.Error(errors.Errorf("msg protocol should be 'bar'"))
	}
}

func TestChannelAckUnmarshal(t *testing.T) {
	rawMsg := []byte{0x02}
	msgUncast, err := Parse(rawMsg)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	_, ok := msgUncast.(*ChannelAck)
	if !ok {
		t.Error(errors.Errorf("Failed to cast to ChannelAck"))
	}
}
