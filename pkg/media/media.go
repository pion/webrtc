// Package media provides media writer and filters
package media

import (
	"time"

	"github.com/pion/rtp"
)

// A Sample contains encoded media and the number of samples in that media (see NSamples).
type Sample struct {
	Data    []byte
	Samples uint32
}

// NSamples calculates the number of samples in media of length d with sampling frequency f.
// For example, NSamples(20 * time.Millisecond, 48000) will return the number of samples
// in a 20 millisecond segment of Opus audio recorded at 48000 samples per second.
func NSamples(d time.Duration, freq int) uint32 {
	return uint32(time.Duration(freq) * d / time.Second)
}

// Writer defines an interface to handle
// the creation of media files
type Writer interface {
	// Add the content of an RTP packet to the media
	WriteRTP(packet *rtp.Packet) error
	// Close the media
	// Note: Close implementation must be idempotent
	Close() error
}
