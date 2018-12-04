package rtcp

import (
	"reflect"
	"testing"
)

func TestSliceLossIndicationUnmarshal(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Data      []byte
		Want      SliceLossIndication
		WantError error
	}{
		{
			Name: "valid",
			Data: []byte{
				// SliceLossIndication
				0x82, 0xcd, 0x0, 0x3,
				// sender=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// media=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// nack 0xAAAA, 0x5555
				0x55, 0x50, 0x00, 0x2C,
			},
			Want: SliceLossIndication{
				SenderSSRC: 0x902f9e2e,
				MediaSSRC:  0x902f9e2e,
				SLI:        []SLIEntry{{0xaaa, 0, 0x2C}},
			},
		},
		{
			Name: "short report",
			Data: []byte{
				0x81, 0xcd, 0x0, 0x2,
				// ssrc=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// report ends early
			},
			WantError: errPacketTooShort,
		},
		{
			Name: "wrong type",
			Data: []byte{
				// v=2, p=0, count=1, SR, len=7
				0x81, 0xc8, 0x0, 0x7,
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
			},
			WantError: errWrongType,
		},
		{
			Name:      "nil",
			Data:      nil,
			WantError: errPacketTooShort,
		},
	} {
		var sli SliceLossIndication
		err := sli.Unmarshal(test.Data)
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Unmarshal %q rr: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		if got, want := sli, test.Want; !reflect.DeepEqual(got, want) {
			t.Fatalf("Unmarshal %q rr: got %v, want %v", test.Name, got, want)
		}
	}
}

func TestSliceLossIndicationRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Report    SliceLossIndication
		WantError error
	}{
		{
			Name: "valid",
			Report: SliceLossIndication{
				SenderSSRC: 0x902f9e2e,
				MediaSSRC:  0x902f9e2e,
				SLI:        []SLIEntry{{1, 0xAA, 0x1F}, {1034, 0x05, 0x6}},
			},
		},
	} {
		data, err := test.Report.Marshal()
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Marshal %q: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		var decoded SliceLossIndication
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatalf("Unmarshal %q: %v", test.Name, err)
		}

		if got, want := decoded, test.Report; !reflect.DeepEqual(got, want) {
			t.Fatalf("%q sli round trip: got %#v, want %#v", test.Name, got, want)
		}
	}
}
