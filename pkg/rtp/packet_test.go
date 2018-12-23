package rtp

import (
	"reflect"
	"testing"
)

func TestBasic(t *testing.T) {
	p := &Packet{}

	if err := p.Unmarshal([]byte{}); err == nil {
		t.Fatal("Unmarshal did not error on zero length packet")
	}

	rawPkt := []byte{
		0x80, 0x60, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
		0x27, 0x82, 0x98, 0x36, 0xbe, 0x88, 0x9e,
	}
	parsedPacket := &Packet{
		Raw:            rawPkt,
		Version:        2,
		PayloadOffset:  12,
		PayloadType:    96,
		SequenceNumber: 27023,
		Timestamp:      3653407706,
		SSRC:           476325762,
		Payload:        rawPkt[12:],
		CSRC:           []uint32{},
	}

	if err := p.Unmarshal(rawPkt); err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(p, parsedPacket) {
		t.Errorf("TestBasic unmarshal: got %#v, want %#v", p, parsedPacket)
	}

	raw, err := p.Marshal()
	if err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(raw, rawPkt) {
		t.Errorf("TestBasic marshal: got %#v, want %#v", raw, rawPkt)
	}
}
