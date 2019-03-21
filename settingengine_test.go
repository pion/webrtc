// +build !js

package webrtc

import (
	"testing"
	"time"
)

func TestSetEphemeralUDPPortRange(t *testing.T) {
	s := SettingEngine{}

	if s.ephemeralUDP.PortMin != 0 ||
		s.ephemeralUDP.PortMax != 0 {
		t.Fatalf("SettingEngine defaults aren't as expected.")
	}

	// set bad ephemeral ports
	if err := s.SetEphemeralUDPPortRange(3000, 2999); err == nil {
		t.Fatalf("Setting engine should fail bad ephemeral ports.")
	}

	if err := s.SetEphemeralUDPPortRange(3000, 4000); err != nil {
		t.Fatalf("Setting engine failed valid port range: %s", err)
	}

	if s.ephemeralUDP.PortMin != 3000 ||
		s.ephemeralUDP.PortMax != 4000 {
		t.Fatalf("Setting engine ports do not reflect expected range")
	}
}

func TestSetConnectionTimeout(t *testing.T) {
	s := SettingEngine{}

	if s.timeout.ICEConnection != nil ||
		s.timeout.ICEKeepalive != nil {
		t.Fatalf("SettingEngine defaults aren't as expected.")
	}

	s.SetConnectionTimeout(5*time.Second, 1*time.Second)

	if s.timeout.ICEConnection == nil ||
		*s.timeout.ICEConnection != 5*time.Second ||
		s.timeout.ICEKeepalive == nil ||
		*s.timeout.ICEKeepalive != 1*time.Second {
		t.Fatalf("ICE Timeouts do not reflect requested values.")
	}
}

func TestDetachDataChannels(t *testing.T) {
	s := SettingEngine{}

	if s.detach.DataChannels {
		t.Fatalf("SettingEngine defaults aren't as expected.")
	}

	s.DetachDataChannels()

	if !s.detach.DataChannels {
		t.Fatalf("Failed to enable detached data channels.")
	}
}
