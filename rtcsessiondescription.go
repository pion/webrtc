package webrtc

import (
	"github.com/pions/webrtc/internal/sdp"
)

// RTCSessionDescription is used to expose local and remote session descriptions.
type RTCSessionDescription struct {
	Type RTCSdpType
	Sdp  string

	// This will never be initialized by callers, internal use only
	parsed *sdp.SessionDescription
}