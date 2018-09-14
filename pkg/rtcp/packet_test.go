package rtcp

import (
	"reflect"
	"testing"
)

// An RTCP packet from a packet dump
var realPacket = []byte{
	// Header (offset=0)
	// v=1, p=0, count=1, RR, len=7
	0x81, 0xc9, 0x0, 0x7,

	// Receiver Report (offset=4)
	// ssrc=0x902f9e2e
	0x90, 0x2f, 0x9e, 0x2e,
	// ssrc=0xbc5e9a40
	0xbc, 0x5e, 0x9a, 0x40,
	// fracLost=0, totalLost=0
	0x0, 0x0, 0x0, 0x0,
	// lastSeq=0x46e1
	0x0, 0x0, 0x46, 0xe1,
	// jitter=273
	0x0, 0x0, 0x1, 0x11,
	// lsr=0x9f36432
	0x9, 0xf3, 0x64, 0x32,
	// delay=150137
	0x0, 0x2, 0x4a, 0x79,

	// Header (offset=32)
	// v=1, p=0, count=1, SDES, len=12
	0x81, 0xca, 0x0, 0xc,

	// Source Description (offset=36)
	// ssrc=0x902f9e2e
	0x90, 0x2f, 0x9e, 0x2e,
	// CNAME, len=38
	0x1, 0x26,
	// text="{9c00eb92-1afb-9d49-a47d-91f64eee69f5}"
	0x7b, 0x39, 0x63, 0x30,
	0x30, 0x65, 0x62, 0x39,
	0x32, 0x2d, 0x31, 0x61,
	0x66, 0x62, 0x2d, 0x39,
	0x64, 0x34, 0x39, 0x2d,
	0x61, 0x34, 0x37, 0x64,
	0x2d, 0x39, 0x31, 0x66,
	0x36, 0x34, 0x65, 0x65,
	0x65, 0x36, 0x39, 0x66,
	0x35, 0x7d,
	// END + padding
	0x0, 0x0, 0x0, 0x0,

	// Header (offset=84)
	// v=1, p=0, count=1, BYE, len=1
	0x81, 0xcb, 0x0, 0x1,
	// source=0x902f9e2e
	0x90, 0x2f, 0x9e, 0x2e,
}

func TestUnmarshal(t *testing.T) {
	// TODO: write a Packet class to make parsing multiple packets easier

	var offset uint16

	// Get header
	wantHeader := Header{
		Version: 2,
		Padding: false,
		Count:   1,
		Type:    TypeReceiverReport,
		Length:  7,
	}
	var header Header
	if err := header.Unmarshal(realPacket[offset:]); err != nil {
		t.Errorf("Unmarshal: %v", err)
	}
	if got, want := wantHeader, header; !reflect.DeepEqual(got, want) {
		t.Errorf("Unmarshal: got %#v, want %#v", got, want)
	}

	// Get RR
	pktLen := (header.Length + 1) * 4
	rrData := realPacket[headerLength:pktLen]

	var rr ReceiverReport
	if err := rr.Unmarshal(rrData); err != nil {
		t.Errorf("Unmarshal: %v", err)
	}
	wantRR := ReceiverReport{
		SSRC: 0x902f9e2e,
		Reports: []ReceptionReport{{
			SSRC:               0xbc5e9a40,
			FractionLost:       0,
			TotalLost:          0,
			LastSequenceNumber: 0x46e1,
			Jitter:             273,
			LastSenderReport:   0x9f36432,
			Delay:              150137,
		}},
	}
	if got, want := wantRR, rr; !reflect.DeepEqual(got, want) {
		t.Errorf("Unmarshal: got %#v, want %#v", got, want)
	}

	offset += pktLen

	// Get Header
	if err := header.Unmarshal(realPacket[offset:]); err != nil {
		t.Errorf("Unmarshal: %v", err)
	}
	wantHeader = Header{
		Version: 2,
		Padding: false,
		Count:   1,
		Type:    TypeSourceDescription,
		Length:  12,
	}
	if got, want := header, wantHeader; !reflect.DeepEqual(got, want) {
		t.Errorf("Unmarshal: got %#v, want %#v", got, want)
	}

	// Get SDES
	pktLen = (header.Length + 1) * 4
	sdesData := realPacket[offset+headerLength : offset+pktLen]

	var sdes SourceDescription
	if err := sdes.Unmarshal(sdesData); err != nil {
		t.Errorf("Unmarshal: %v", err)
	}
	wantSdes := SourceDescription{
		Chunks: []SourceDescriptionChunk{
			{
				Source: 0x902f9e2e,
				Items: []SourceDescriptionItem{
					{
						Type: SDESCNAME,
						Text: "{9c00eb92-1afb-9d49-a47d-91f64eee69f5}",
					},
				},
			},
		},
	}
	if got, want := sdes, wantSdes; !reflect.DeepEqual(got, want) {
		t.Errorf("sdes: got %#v, want %#v", got, want)
	}

	offset += pktLen

	// Get header
	if err := header.Unmarshal(realPacket[offset:]); err != nil {
		t.Errorf("Unmarshal: %v", err)
	}
	wantHeader = Header{
		Version: 2,
		Padding: false,
		Count:   1,
		Type:    TypeGoodbye,
		Length:  1,
	}
	if got, want := wantHeader, header; !reflect.DeepEqual(got, want) {
		t.Errorf("Unmarshal: got %#v, want %#v", got, want)
	}
}
