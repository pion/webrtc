package webrtc

import (
	"time"

	"github.com/pions/webrtc/pkg/ice"
)

// ICEGatherOptions provides options relating to the gathering of ICE candidates.
type ICEGatherOptions struct {
	ICEServers []ICEServer
}

// ICEAgentOptions contains non-standard options that can be passed to NewICEGatherer
// to change the behavior of the ICE agent or access lower-level features.
type ICEAgentOptions struct {
	PortMin           uint16
	PortMax           uint16
	ConnectionTimeout time.Duration
	KeepaliveInterval time.Duration
}

// Validate returns an error if the IceAgentOptions are invalid and cannot be used,
// or nil otherwise.
func (i *ICEAgentOptions) Validate() error {
	if i.PortMax < i.PortMin {
		return ice.ErrPort
	}
	return nil
}
