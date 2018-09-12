package rtcp

import (
	"reflect"
	"testing"
)

// An RTCP packet from a packet dump
var realPacket = []byte{129, 201, 0, 7, 144, 47, 158, 46, 188, 94, 154, 64, 0, 0, 0, 0, 0, 0, 70, 225, 0, 0, 1, 17, 9, 243, 100, 50, 0, 2, 74, 121, 129, 202, 0, 12, 144, 47, 158, 46, 1, 38, 123, 57, 99, 48, 48, 101, 98, 57, 50, 45, 49, 97, 102, 98, 45, 57, 100, 52, 57, 45, 97, 52, 55, 100, 45, 57, 49, 102, 54, 52, 101, 101, 101, 54, 57, 102, 53, 125, 0, 0, 0, 0, 129, 203, 0, 1, 144, 47, 158, 46}

func TestHeaderUnmarshal(t *testing.T) {
	data := make([]byte, headerLength)
	copy(data, realPacket)

	want := Header{
		Version:     2,
		Padding:     false,
		ReportCount: 1,
		Type:        TypeReceiverReport,
		Length:      7,
	}

	var got Header
	if err := got.Unmarshal(data); err != nil {
		t.Errorf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Unmarshal: got %#v, want %#v", got, want)
	}
}

func TestHeaderUnmarshalNil(t *testing.T) {
	var header Header
	err := header.Unmarshal(nil)
	if got, want := err, errPacketTooShort; got != want {
		t.Fatalf("unmarshal nil header: err = %v, want %v", got, want)
	}
}
func TestHeaderRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Header    Header
		WantError error
	}{
		{
			Name: "valid",
			Header: Header{
				Version: 2,
				Padding: true,
				Count:   31,
				Type:    TypeSenderReport,
				Length:  4,
			},
		},
		{
			Name: "also valid",
			Header: Header{
				Version: 1,
				Padding: false,
				Count:   28,
				Type:    TypeReceiverReport,
				Length:  65535,
			},
		},
		{
			Name: "invalid version",
			Header: Header{
				Version: 99,
			},
			WantError: errInvalidVersion,
		},
		{
			Name: "invalid count",
			Header: Header{
				Count: 40,
			},
			WantError: errInvalidCount,
		},
	} {
		data, err := test.Header.Marshal()
		if got, want := err, test.WantError; got != want {
			t.Errorf("Marshal %q: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		var decoded Header
		if err := decoded.Unmarshal(data); err != nil {
			t.Errorf("Unmarshal %q: %v", test.Name, err)
		}

		if got, want := decoded, test.Header; !reflect.DeepEqual(got, want) {
			t.Errorf("%q header round trip: got %#v, want %#v", test.Name, got, want)
		}
	}
}
