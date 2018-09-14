package rtcp

import (
	"reflect"
	"testing"
)

func TestReceiverReportUnmarshalNil(t *testing.T) {
	var rr ReceiverReport
	err := rr.Unmarshal(nil)
	if got, want := err, errPacketTooShort; got != want {
		t.Fatalf("unmarshal nil rr: err = %v, want %v", got, want)
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
	} {
		data, err := test.Report.Marshal()
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Marshal %q: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		var decoded ReceiverReport
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatalf("Unmarshal %q: %v", test.Name, err)
		}

		if got, want := decoded, test.Report; !reflect.DeepEqual(got, want) {
			t.Fatalf("%q rr round trip: got %#v, want %#v", test.Name, got, want)
		}
	}
}
