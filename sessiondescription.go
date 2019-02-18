package webrtc

import (
	"github.com/pions/sdp/v2"
)

// SessionDescription is used to expose local and remote session descriptions.
type SessionDescription struct {
	Type SDPType `json:"type"`
	SDP  string  `json:"sdp"`

	// This will never be initialized by callers, internal use only
	parsed *sdp.SessionDescription
}
