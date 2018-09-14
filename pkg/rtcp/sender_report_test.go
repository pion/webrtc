package rtcp

import (
	"reflect"
	"testing"
)

func TestSenderReportUnmarshalNil(t *testing.T) {
	var sr SenderReport
	err := sr.Unmarshal(nil)
	if got, want := err, errPacketTooShort; got != want {
		t.Fatalf("unmarshal nil sr: err = %v, want %v", got, want)
	}
}

func TestSenderReportRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Report    SenderReport
		WantError error
	}{
		{
			Name: "valid",
			Report: SenderReport{
				SSRC:        1,
				NTPTime:     999,
				RTPTime:     555,
				PacketCount: 32,
				OctetCount:  11,
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
			Report: SenderReport{
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
	} {
		data, err := test.Report.Marshal()
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Marshal %q: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		var decoded SenderReport
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatalf("Unmarshal %q: %v", test.Name, err)
		}

		if got, want := decoded, test.Report; !reflect.DeepEqual(got, want) {
			t.Fatalf("%q sr round trip: got %#v, want %#v", test.Name, got, want)
		}
	}
}
