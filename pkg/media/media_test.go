package media_test

import (
	"testing"
	"time"

	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
)

func TestNSamples(t *testing.T) {
	assert.Equal(t, media.NSamples(20*time.Millisecond, 48000), uint32(48000*0.02))
}
