// +build !js

package webrtc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestSetNAT1To1IPs(t *testing.T) {
	s := SettingEngine{}
	if s.candidates.NAT1To1IPs != nil {
		t.Errorf("Invalid default value")
	}
	if s.candidates.NAT1To1IPCandidateType != 0 {
		t.Errorf("Invalid default value")
	}

	ips := []string{"1.2.3.4"}
	typ := ICECandidateTypeHost
	s.SetNAT1To1IPs(ips, typ)
	if len(s.candidates.NAT1To1IPs) != 1 || s.candidates.NAT1To1IPs[0] != "1.2.3.4" {
		t.Fatalf("Failed to set NAT1To1IPs")
	}
	if s.candidates.NAT1To1IPCandidateType != typ {
		t.Fatalf("Failed to set NAT1To1IPCandidateType")
	}
}

func TestSetAnsweringDTLSRole(t *testing.T) {
	s := SettingEngine{}
	assert.Error(t, s.SetAnsweringDTLSRole(DTLSRoleAuto), "SetAnsweringDTLSRole can only be called with DTLSRoleClient or DTLSRoleServer")
	assert.Error(t, s.SetAnsweringDTLSRole(DTLSRole(0)), "SetAnsweringDTLSRole can only be called with DTLSRoleClient or DTLSRoleServer")
}

func TestSetReplayProtection(t *testing.T) {
	s := SettingEngine{}

	if s.replayProtection.DTLS != nil ||
		s.replayProtection.SRTP != nil ||
		s.replayProtection.SRTCP != nil {
		t.Fatalf("SettingEngine defaults aren't as expected.")
	}

	s.SetDTLSReplayProtectionWindow(128)
	s.SetSRTPReplayProtectionWindow(64)
	s.SetSRTCPReplayProtectionWindow(32)

	if s.replayProtection.DTLS == nil ||
		*s.replayProtection.DTLS != 128 {
		t.Errorf("Failed to set DTLS replay protection window")
	}
	if s.replayProtection.SRTP == nil ||
		*s.replayProtection.SRTP != 64 {
		t.Errorf("Failed to set SRTP replay protection window")
	}
	if s.replayProtection.SRTCP == nil ||
		*s.replayProtection.SRTCP != 32 {
		t.Errorf("Failed to set SRTCP replay protection window")
	}
}
