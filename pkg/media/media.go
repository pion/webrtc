// Package media provides media writer and filters
package media

import (
	"time"

	"github.com/pion/rtp"
)

// SampleHyperscaleExtensions contains information about the sample that is required for hyperscale
type SampleHyperscaleExtensions struct {
	Don                      uint16
	IsIFrame                 bool
	IsSpsPps                 bool
	IsAbr                    bool
	WithHyperscaleExtensions bool
}

// A Sample contains encoded media and timing information
type Sample struct {
	Data               []byte
	Timestamp          time.Time
	Duration           time.Duration
	PacketTimestamp    uint32
	PrevDroppedPackets uint16
	Extensions         []rtp.Extension
	SampleHyperscaleExtensions
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
