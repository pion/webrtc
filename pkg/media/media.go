// Package media provides media writer and filters
package media

import (
	"github.com/pion/rtp"
)

// Sample contains media, and the amount of samples in it
type Sample struct {
	Data    []byte
	Samples uint32
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
