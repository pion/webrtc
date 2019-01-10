package ntp

import (
	"testing"
	"time"
)

func TestEra(t *testing.T) {
	for _, test := range []struct {
		Time time.Time
		Want int32
	}{
		{
			Time: time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
			Want: 0,
		},
		{
			Time: time.Date(1850, 1, 1, 0, 0, 0, 0, time.UTC),
			Want: -1,
		},
		{
			Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			Want: 0,
		},
		{
			Time: time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC),
			Want: 1,
		},
	} {
		if got, want := era(test.Time), test.Want; got != want {
			t.Fatalf("era(%v) = %v, want %v", test.Time, got, want)
		}
	}
}

func TestTime64(t *testing.T) {
	for _, test := range []struct {
		Time64 Time64
		Want   time.Time
	}{
		{
			Time64: Time64(0xDA8BD1fCDDDDA05A),
			Want:   time.Date(2016, 3, 10, 10, 59, 8, 866663000, time.UTC),
		},
	} {
		if got, want := test.Time64.Time(), test.Want; got != want {
			t.Fatalf("Time() = %v, want %v", got, want)
		}
	}
}
