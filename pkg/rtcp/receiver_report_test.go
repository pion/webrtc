package rtcp

import (
	"reflect"
	"testing"
)

func TestReceiverReportUnmarshal(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Data      []byte
		Want      ReceiverReport
		WantError error
	}{
		{
			Name: "valid",
			Data: []byte{
				// v=2, p=0, count=1, RR, len=7
				0x81, 0xc9, 0x0, 0x7,
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
			Want: ReceiverReport{
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
				ProfileExtensions: []byte{},
			},
		},
		{
			Name: "valid with extension data",
			Data: []byte{
				// v=2, p=0, count=1, RR, len=9
				0x81, 0xc9, 0x0, 0x9,
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
				// profile-specific extension data
				0x54, 0x45, 0x53, 0x54,
				0x44, 0x41, 0x54, 0x41,
			},
			Want: ReceiverReport{
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
				ProfileExtensions: []byte{
					0x54, 0x45, 0x53, 0x54,
					0x44, 0x41, 0x54, 0x41},
			},
		},
		{
			Name: "short report",
			Data: []byte{
				// v=2, p=0, count=1, RR, len=7
				0x81, 0xc9, 0x00, 0x0c,
				// ssrc=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// fracLost=0, totalLost=0
				0x00, 0x00, 0x00, 0x00,
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
			Name: "bad count in header",
			Data: []byte{
				// v=2, p=0, count=2, RR, len=7
				0x82, 0xc9, 0x0, 0x7,
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
			WantError: errInvalidHeader,
		},
		{
			Name:      "nil",
			Data:      nil,
			WantError: errPacketTooShort,
		},
	} {
		var rr ReceiverReport
		err := rr.Unmarshal(test.Data)
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Unmarshal %q rr: err = %#v, want %#v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		if got, want := rr, test.Want; !reflect.DeepEqual(got, want) {
			t.Fatalf("Unmarshal %q rr: got %#v, want %#v", test.Name, got, want)
		}
	}
}

func TestReceiverReportRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Report    ReceiverReport
		WantError error
	}{
		{
			Name: "valid",
			Report: ReceiverReport{
				SSRC: 1,
				Reports: []ReceptionReport{
					{
						SSRC:               2,
						FractionLost:       2,
						TotalLost:          3,
						LastSequenceNumber: 4,
						Jitter:             5,
						LastSenderReport:   6,
						Delay:              7,
					},
					{
						SSRC: 0,
					},
				},
				ProfileExtensions: []byte{},
			},
		},
		{
			Name: "also valid",
			Report: ReceiverReport{
				SSRC: 2,
				Reports: []ReceptionReport{
					{
						SSRC:               999,
						FractionLost:       30,
						TotalLost:          12345,
						LastSequenceNumber: 99,
						Jitter:             22,
						LastSenderReport:   92,
						Delay:              46,
					},
				},
				ProfileExtensions: []byte{},
			},
		},
		{
			Name: "totallost overflow",
			Report: ReceiverReport{
				SSRC: 1,
				Reports: []ReceptionReport{{
					TotalLost: 1 << 25,
				}},
			},
			WantError: errInvalidTotalLost,
		},
		{
			Name: "count overflow",
			Report: ReceiverReport{
				SSRC:    1,
				Reports: tooManyReports,
			},
			WantError: errTooManyReports,
		},
	} {
		data, err := test.Report.Marshal()
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Marshal %q: err = %#v, want %#v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		var decoded ReceiverReport
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatalf("Unmarshal %q: %#v", test.Name, err)
		}

		if got, want := decoded, test.Report; !reflect.DeepEqual(got, want) {
			t.Fatalf("%q rr round trip: got %#v, want %#v", test.Name, got, want)
		}
	}
}

// a slice with enough ReceptionReports to overflow an 5-bit int
var tooManyReports []ReceptionReport

func init() {
	for i := 0; i < (1 << 5); i++ {
		tooManyReports = append(tooManyReports, ReceptionReport{
			SSRC:               2,
			FractionLost:       2,
			TotalLost:          3,
			LastSequenceNumber: 4,
			Jitter:             5,
			LastSenderReport:   6,
			Delay:              7,
		})
	}
}
