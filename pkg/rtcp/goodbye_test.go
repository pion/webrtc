package rtcp

import (
	"reflect"
	"testing"
)

func TestGoodbyeUnmarshal(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Data      []byte
		Want      Goodbye
		WantError error
	}{
		{
			Name: "valid",
			Data: []byte{
				// v=2, p=0, count=1, BYE, len=12
				0x81, 0xcb, 0x00, 0x0c,
				// ssrc=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// len=3, text=FOO
				0x03, 0x46, 0x4f, 0x4f,
			},
			Want: Goodbye{
				Sources: []uint32{0x902f9e2e},
				Reason:  "FOO",
			},
		},
		{
			Name: "invalid octet count",
			Data: []byte{
				// v=2, p=0, count=1, BYE, len=12
				0x81, 0xcb, 0x00, 0x0c,
				// ssrc=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// len=4, text=FOO
				0x04, 0x46, 0x4f, 0x4f,
			},
			WantError: errPacketTooShort,
		},
		{
			Name: "wrong type",
			Data: []byte{
				// v=2, p=0, count=1, SDES, len=12
				0x81, 0xca, 0x00, 0x0c,
				// ssrc=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// len=3, text=FOO
				0x03, 0x46, 0x4f, 0x4f,
			},
			WantError: errWrongType,
		},
		{
			Name: "short reason",
			Data: []byte{
				// v=2, p=0, count=1, BYE, len=12
				0x81, 0xcb, 0x00, 0x0c,
				// ssrc=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// len=3, text=F + padding
				0x01, 0x46, 0x00, 0x00,
			},
			Want: Goodbye{
				Sources: []uint32{0x902f9e2e},
				Reason:  "F",
			},
		},
		{
			Name: "not byte aligned",
			Data: []byte{
				// v=2, p=0, count=1, BYE, len=10
				0x81, 0xcb, 0x00, 0x0a,
				// ssrc=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// len=1, text=F
				0x01, 0x46,
			},
			WantError: errPacketTooShort,
		},
		{
			Name: "bad count in header",
			Data: []byte{
				// v=2, p=0, count=2, BYE, len=8
				0x82, 0xcb, 0x00, 0x0c,
				// ssrc=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
			},
			WantError: errPacketTooShort,
		},
		{
			Name: "empty packet",
			Data: []byte{
				// v=2, p=0, count=0, BYE, len=4
				0x80, 0xcb, 0x00, 0x04,
			},
			Want: Goodbye{
				Sources: []uint32{},
				Reason:  "",
			},
		},
		{
			Name:      "nil",
			Data:      nil,
			WantError: errInvalidHeader,
		},
	} {
		var bye Goodbye
		err := bye.Unmarshal(test.Data)
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Unmarshal %q bye: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		if got, want := bye, test.Want; !reflect.DeepEqual(got, want) {
			t.Fatalf("Unmarshal %q bye: got %v, want %v", test.Name, got, want)
		}
	}
}
func TestGoodbyeRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Bye       Goodbye
		WantError error
	}{
		{
			Name: "empty",
			Bye: Goodbye{
				Sources: []uint32{},
			},
		},
		{
			Name: "valid",
			Bye: Goodbye{
				Sources: []uint32{
					0x01020304,
					0x05060708,
				},
				Reason: "because",
			},
		},
		{
			Name: "empty reason",
			Bye: Goodbye{
				Sources: []uint32{0x01020304},
				Reason:  "",
			},
		},
		{
			Name: "reason no source",
			Bye: Goodbye{
				Sources: []uint32{},
				Reason:  "foo",
			},
		},
		{
			Name: "short reason",
			Bye: Goodbye{
				Sources: []uint32{},
				Reason:  "f",
			},
		},
		{
			Name: "count overflow",
			Bye: Goodbye{
				Sources: tooManySources,
			},
			WantError: errTooManySources,
		},
		{
			Name: "reason too long",
			Bye: Goodbye{
				Sources: []uint32{},
				Reason:  tooLongText,
			},
			WantError: errReasonTooLong,
		},
	} {
		data, err := test.Bye.Marshal()
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Marshal %q: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		var bye Goodbye
		if err := bye.Unmarshal(data); err != nil {
			t.Fatalf("Unmarshal %q: %v", test.Name, err)
		}

		if got, want := bye, test.Bye; !reflect.DeepEqual(got, want) {
			t.Fatalf("%q sdes round trip: got %#v, want %#v", test.Name, got, want)
		}
	}
}

// a slice with enough sources to overflow an 5-bit int
var tooManySources []uint32

func init() {
	for i := 0; i < (1 << 5); i++ {
		tooManySources = append(tooManySources, 0x00)
	}
}
