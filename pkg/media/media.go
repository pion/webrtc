package media

import (
	"github.com/pions/rtp"
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
	AddPacket(packet *rtp.Packet) error
	// Close the media
	// Note: Close implementation must be idempotent
	Close() error
}
