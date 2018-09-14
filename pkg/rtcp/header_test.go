package rtcp

import (
	"reflect"
	"testing"
)

func TestHeaderUnmarshalNil(t *testing.T) {
	var header Header
	err := header.Unmarshal(nil)
	if got, want := err, errInvalidHeader; got != want {
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
			WantError: errInvalidHeader,
		},
		{
			Name: "invalid count",
			Header: Header{
				Count: 40,
			},
			WantError: errInvalidHeader,
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
