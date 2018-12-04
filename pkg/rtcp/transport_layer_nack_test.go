package rtcp

import (
	"reflect"
	"testing"
)

func TestTransportLayerNackUnmarshal(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Data      []byte
		Want      TransportLayerNack
		WantError error
	}{
		{
			Name: "valid",
			Data: []byte{
				// TransportLayerNack
				0x81, 0xcd, 0x0, 0x3,
				// sender=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// media=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// nack 0xAAAA, 0x5555
				0xaa, 0xaa, 0x55, 0x55,
			},
			Want: TransportLayerNack{
				SenderSSRC: 0x902f9e2e,
				MediaSSRC:  0x902f9e2e,
				Nacks:      []NackPair{{0xaaaa, 0x5555}},
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
		var tln TransportLayerNack
		err := tln.Unmarshal(test.Data)
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Unmarshal %q rr: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		if got, want := tln, test.Want; !reflect.DeepEqual(got, want) {
			t.Fatalf("Unmarshal %q rr: got %v, want %v", test.Name, got, want)
		}
	}
}

func TestTransportLayerNackRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Report    TransportLayerNack
		WantError error
	}{
		{
			Name: "valid",
			Report: TransportLayerNack{
				SenderSSRC: 0x902f9e2e,
				MediaSSRC:  0x902f9e2e,
				Nacks:      []NackPair{{1, 0xAA}, {1034, 0x05}},
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

		var decoded TransportLayerNack
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatalf("Unmarshal %q: %v", test.Name, err)
		}

		if got, want := decoded, test.Report; !reflect.DeepEqual(got, want) {
			t.Fatalf("%q tln round trip: got %#v, want %#v", test.Name, got, want)
		}
	}
}
