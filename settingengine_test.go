package webrtc

import (
	"testing"
	"time"
)

func TestSettingEngine(t *testing.T) {
	api := NewAPI()

	if (api.settingEngine.EphemeralUDP.PortMin != 0) ||
		(api.settingEngine.EphemeralUDP.PortMax != 0) ||
		(api.settingEngine.Detach.DataChannels) ||
		(api.settingEngine.Timeout.ICEConnection != nil) ||
		(api.settingEngine.Timeout.ICEKeepalive != nil) {
		t.Fatalf("SettingEngine defaults aren't as expected.")
	}

	//set bad ephemeral ports
	err := api.settingEngine.SetEphemeralUDPPortRange(3000, 2999)
	if err == nil {
		t.Fatalf("Setting engine should fail bad ephemeral ports.")
	}

	err = api.settingEngine.SetEphemeralUDPPortRange(3000, 4000)

	if err != nil {
		t.Fatalf("Setting engine failed valid port range: %s", err)
	}

	if (api.settingEngine.EphemeralUDP.PortMin != 3000) ||
		(api.settingEngine.EphemeralUDP.PortMax != 4000) {
		t.Fatalf("Setting engine ports do not reflect expected range")
	}

	api.settingEngine.DetachDataChannels()

	if !api.settingEngine.Detach.DataChannels {
		t.Fatalf("Datachannels didn't detach when requested")
	}

	api.settingEngine.SetConnectionTimeout(5*time.Second, 1*time.Second)

	if (api.settingEngine.Timeout.ICEConnection == nil) ||
		(*api.settingEngine.Timeout.ICEConnection != 5*time.Second) ||
		(api.settingEngine.Timeout.ICEKeepalive == nil) ||
		(*api.settingEngine.Timeout.ICEKeepalive != 1*time.Second) {
		t.Fatalf("ICE Timeouts do not reflect requested values.")
	}
}
