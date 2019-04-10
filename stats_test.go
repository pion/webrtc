package webrtc

import (
	"testing"
	"time"
)

func TestStatsTimestampTime(t *testing.T) {
	for _, test := range []struct {
		Timestamp StatsTimestamp
		WantTime  time.Time
	}{
		{
			Timestamp: 0,
			WantTime:  time.Unix(0, 0),
		},
		{
			Timestamp: 1,
			WantTime:  time.Unix(0, 1e6),
		},
		{
			Timestamp: 0.001,
			WantTime:  time.Unix(0, 1e3),
		},
	} {
		if got, want := test.Timestamp.Time(), test.WantTime.UTC(); got != want {
			t.Fatalf("StatsTimestamp(%v).Time() = %v, want %v", test.Timestamp, got, want)
		}
	}
}
