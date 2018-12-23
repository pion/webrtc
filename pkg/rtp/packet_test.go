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
		0x90, 0x60, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
		0x27, 0x82, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x98, 0x36, 0xbe, 0x88, 0x9e,
	}
	parsedPacket := &Packet{
		Raw:              rawPkt,
		Extension:        true,
		ExtensionProfile: 1,
		ExtensionPayload: []byte{0xFF, 0xFF, 0xFF, 0xFF},
		Version:          2,
		PayloadOffset:    20,
		PayloadType:      96,
		SequenceNumber:   27023,
		Timestamp:        3653407706,
		SSRC:             476325762,
		Payload:          rawPkt[20:],
		CSRC:             []uint32{},
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

func TestExtension(t *testing.T) {
	p := &Packet{}

	missingExtensionPkt := []byte{
		0x90, 0x60, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
		0x27, 0x82,
	}
	if err := p.Unmarshal(missingExtensionPkt); err == nil {
		t.Fatal("Unmarshal did not error on packet with missing extension data")
	}

	invalidExtensionLengthPkt := []byte{
		0x90, 0x60, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
		0x27, 0x82, 0x99, 0x99, 0x99, 0x99,
	}
	if err := p.Unmarshal(invalidExtensionLengthPkt); err == nil {
		t.Fatal("Unmarshal did not error on packet with invalid extension length")
	}

}
