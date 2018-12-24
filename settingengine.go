package webrtc

import "github.com/pions/webrtc/pkg/ice"

var defaultSettingEngine = newSettingEngine()

// SetEphemeralUDPPortRange limits the pool of ephemeral ports that
// ICE UDP connections can allocate from. This setting currently only
// affects host candidates, not server reflexive candidates.
func SetEphemeralUDPPortRange(portMin, portMax uint16) error {
	if portMax < portMin {
		return ice.ErrPort
	}

	defaultSettingEngine.EphemeralUDP.PortMin = portMin
	defaultSettingEngine.EphemeralUDP.PortMax = portMax
	return nil
}

// DetachDataChannels enables detaching data channels. When enabled
// data channels have to be detached in the OnOpen callback using the
// RTCDataChannel.Detach method.
func DetachDataChannels() {
	defaultSettingEngine.DetachDataChannels()
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
}

// DetachDataChannels enables detaching data channels. When enabled
// data channels have to be detached in the OnOpen callback using the
// RTCDataChannel.Detach method.
func (e *settingEngine) DetachDataChannels() {
	e.Detach.DataChannels = true
}

func newSettingEngine() *settingEngine {
	return new(settingEngine)
}
