package media_test

import (
	"testing"
	"time"

	"github.com/pion/webrtc/v2/pkg/media"
)

func TestNSamples(t *testing.T) {
	got := media.NSamples(20*time.Millisecond, 48000)
	want := uint32(48000 * 0.02)
	if got != want {
		t.Errorf("media.NSamples(20*time.Millisecond, 48000)=%v want %v", got, want)
	}
}
