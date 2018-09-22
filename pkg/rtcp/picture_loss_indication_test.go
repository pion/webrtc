package rtcp

import (
	"reflect"
	"testing"
)

func TestPictureLossIndicationUnmarshal(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Data      []byte
		Want      PictureLossIndication
		WantError error
	}{
		{
			Name: "valid",
			Data: []byte{
				// v=2, p=0, FMT=1, PSFB, len=1
				0x81, 0xce, 0x00, 0x02,
				// ssrc=0x0
				0x00, 0x00, 0x00, 0x00,
				// ssrc=0x4bc4fcb4
				0x4b, 0xc4, 0xfc, 0xb4,
			},
			Want: PictureLossIndication{
				SenderSSRC: 0x0,
				MediaSSRC:  0x4bc4fcb4,
			},
		},
		{
			Name: "packet too short",
			Data: []byte{
				0x00, 0x00, 0x00, 0x00,
			},
			WantError: errPacketTooShort,
		},
		{
			Name: "invalid header",
			Data: []byte{
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
			},
			WantError: errBadVersion,
		},
		{
			Name: "wrong type",
			Data: []byte{
				// v=2, p=0, FMT=1, RR, len=1
				0x81, 0xc9, 0x00, 0x02,
				// ssrc=0x0
				0x00, 0x00, 0x00, 0x00,
				// ssrc=0x4bc4fcb4
				0x4b, 0xc4, 0xfc, 0xb4,
			},
			WantError: errWrongType,
		},
		{
			Name: "wrong fmt",
			Data: []byte{
				// v=2, p=0, FMT=2, RR, len=1
				0x82, 0xc9, 0x00, 0x02,
				// ssrc=0x0
				0x00, 0x00, 0x00, 0x00,
				// ssrc=0x4bc4fcb4
				0x4b, 0xc4, 0xfc, 0xb4,
			},
			WantError: errWrongType,
		},
	} {
		var pli PictureLossIndication
		err := pli.Unmarshal(test.Data)
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Unmarshal %q rr: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		if got, want := pli, test.Want; !reflect.DeepEqual(got, want) {
			t.Fatalf("Unmarshal %q rr: got %v, want %v", test.Name, got, want)
		}
	}
}

func TestPictureLossIndicationRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Packet    PictureLossIndication
		WantError error
	}{
		{
			Name: "valid",
			Packet: PictureLossIndication{
				SenderSSRC: 1,
				MediaSSRC:  2,
			},
		},
		{
			Name: "also valid",
			Packet: PictureLossIndication{
				SenderSSRC: 5000,
				MediaSSRC:  6000,
			},
		},
	} {
		data, err := test.Packet.Marshal()
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Marshal %q: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		var decoded PictureLossIndication
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatalf("Unmarshal %q: %v", test.Name, err)
		}

		if got, want := decoded, test.Packet; !reflect.DeepEqual(got, want) {
			t.Fatalf("%q rr round trip: got %#v, want %#v", test.Name, got, want)
		}
	}
}
