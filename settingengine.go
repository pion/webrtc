package webrtc

import (
	"time"

	"github.com/pions/webrtc/pkg/ice"
)

// SetEphemeralUDPPortRange limits the pool of ephemeral ports that
// ICE UDP connections can allocate from. This setting currently only
// affects host candidates, not server reflexive candidates.
func SetEphemeralUDPPortRange(portMin, portMax uint16) error {
	return defaultAPI.settingEngine.SetEphemeralUDPPortRange(portMin, portMax)
}

// DetachDataChannels enables detaching data channels. When enabled
// data channels have to be detached in the OnOpen callback using the
// RTCDataChannel.Detach method.
func DetachDataChannels() {
	defaultAPI.settingEngine.DetachDataChannels()
}

// SetConnectionTimeout sets the amount of silence needed on a given candidate pair
// before the ICE agent considers the pair timed out.
func SetConnectionTimeout(connectionTimeout, keepAlive time.Duration) {
	defaultAPI.settingEngine.SetConnectionTimeout(connectionTimeout, keepAlive)
}

// settingEngine allows influencing behavior in ways that are not
// supported by the WebRTC API. This allows us to support additional
// use-cases without deviating from the WebRTC API elsewhere.
type settingEngine struct {
	EphemeralUDP struct {
		PortMin uint16
		PortMax uint16
	}
	Detach struct {
		DataChannels bool
	}
	Timeout struct {
		ICEConnection *time.Duration
		ICEKeepalive  *time.Duration
	}
}

// DetachDataChannels enables detaching data channels. When enabled
// data channels have to be detached in the OnOpen callback using the
// RTCDataChannel.Detach method.
func (e *settingEngine) DetachDataChannels() {
	e.Detach.DataChannels = true
}

// SetConnectionTimeout sets the amount of silence needed on a given candidate pair
// before the ICE agent considers the pair timed out.
func (e *settingEngine) SetConnectionTimeout(connectionTimeout, keepAlive time.Duration) {
	e.Timeout.ICEConnection = &connectionTimeout
	e.Timeout.ICEKeepalive = &keepAlive
}

// SetEphemeralUDPPortRange limits the pool of ephemeral ports that
// ICE UDP connections can allocate from. This setting currently only
// affects host candidates, not server reflexive candidates.
func (e *settingEngine) SetEphemeralUDPPortRange(portMin, portMax uint16) error {
	if portMax < portMin {
		return ice.ErrPort
	}

	e.EphemeralUDP.PortMin = portMin
	e.EphemeralUDP.PortMax = portMax
	return nil
}

func initSettingEngine(s *settingEngine) {
	*s = settingEngine{}
}
