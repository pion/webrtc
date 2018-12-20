package webrtc

import "github.com/pions/webrtc/pkg/ice"

var defaultSettingEngine = newSettingEngine()

type settingEngine struct {
	EphemeralUDP struct {
		PortMin uint16
		PortMax uint16
	}
}

// SetEphemeralUDPPortRange limits the pool of ephemeral ports that
// ICE UDP connections can allocate from
func SetEphemeralUDPPortRange(portMin, portMax uint16) error {
	if portMax < portMin {
		return ice.ErrPort
	}

	defaultSettingEngine.EphemeralUDP.PortMin = portMin
	defaultSettingEngine.EphemeralUDP.PortMax = portMax
	return nil
}

func newSettingEngine() *settingEngine {
	return new(settingEngine)
}
