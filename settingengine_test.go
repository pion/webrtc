// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/pion/dtls/v2/pkg/crypto/elliptic"
	"github.com/pion/ice/v2"
	"github.com/pion/stun"
	"github.com/pion/transport/v2/test"
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

	var nilDuration *time.Duration
	assert.Equal(t, s.timeout.ICEDisconnectedTimeout, nilDuration)
	assert.Equal(t, s.timeout.ICEFailedTimeout, nilDuration)
	assert.Equal(t, s.timeout.ICEKeepaliveInterval, nilDuration)

	s.SetICETimeouts(1*time.Second, 2*time.Second, 3*time.Second)
	assert.Equal(t, *s.timeout.ICEDisconnectedTimeout, 1*time.Second)
	assert.Equal(t, *s.timeout.ICEFailedTimeout, 2*time.Second)
	assert.Equal(t, *s.timeout.ICEKeepaliveInterval, 3*time.Second)
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

func TestSettingEngine_SetICETCP(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{})
	if err != nil {
		panic(err)
	}

	defer func() {
		_ = listener.Close()
	}()

	tcpMux := NewICETCPMux(nil, listener, 8)

	defer func() {
		_ = tcpMux.Close()
	}()

	settingEngine := SettingEngine{}
	settingEngine.SetICETCPMux(tcpMux)

	assert.Equal(t, tcpMux, settingEngine.iceTCPMux)
}

func TestSettingEngine_SetDisableMediaEngineCopy(t *testing.T) {
	t.Run("Copy", func(t *testing.T) {
		m := &MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())

		api := NewAPI(WithMediaEngine(m))

		offerer, answerer, err := api.newPair(Configuration{})
		assert.NoError(t, err)

		_, err = offerer.AddTransceiverFromKind(RTPCodecTypeVideo)
		assert.NoError(t, err)

		assert.NoError(t, signalPair(offerer, answerer))

		// Assert that the MediaEngine the user created isn't modified
		assert.False(t, m.negotiatedVideo)
		assert.Empty(t, m.negotiatedVideoCodecs)

		// Assert that the internal MediaEngine is modified
		assert.True(t, offerer.api.mediaEngine.negotiatedVideo)
		assert.NotEmpty(t, offerer.api.mediaEngine.negotiatedVideoCodecs)

		closePairNow(t, offerer, answerer)

		newOfferer, newAnswerer, err := api.newPair(Configuration{})
		assert.NoError(t, err)

		// Assert that the first internal MediaEngine hasn't been cleared
		assert.True(t, offerer.api.mediaEngine.negotiatedVideo)
		assert.NotEmpty(t, offerer.api.mediaEngine.negotiatedVideoCodecs)

		// Assert that the new internal MediaEngine isn't modified
		assert.False(t, newOfferer.api.mediaEngine.negotiatedVideo)
		assert.Empty(t, newAnswerer.api.mediaEngine.negotiatedVideoCodecs)

		closePairNow(t, newOfferer, newAnswerer)
	})

	t.Run("No Copy", func(t *testing.T) {
		m := &MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())

		s := SettingEngine{}
		s.DisableMediaEngineCopy(true)

		api := NewAPI(WithMediaEngine(m), WithSettingEngine(s))

		offerer, answerer, err := api.newPair(Configuration{})
		assert.NoError(t, err)

		_, err = offerer.AddTransceiverFromKind(RTPCodecTypeVideo)
		assert.NoError(t, err)

		assert.NoError(t, signalPair(offerer, answerer))

		// Assert that the user MediaEngine was modified, so no copy happened
		assert.True(t, m.negotiatedVideo)
		assert.NotEmpty(t, m.negotiatedVideoCodecs)

		closePairNow(t, offerer, answerer)

		offerer, answerer, err = api.newPair(Configuration{})
		assert.NoError(t, err)

		// Assert that the new internal MediaEngine was modified, so no copy happened
		assert.True(t, offerer.api.mediaEngine.negotiatedVideo)
		assert.NotEmpty(t, offerer.api.mediaEngine.negotiatedVideoCodecs)

		closePairNow(t, offerer, answerer)
	})
}

func TestSetDTLSRetransmissionInterval(t *testing.T) {
	s := SettingEngine{}

	if s.dtls.retransmissionInterval != 0 {
		t.Fatalf("SettingEngine defaults aren't as expected.")
	}

	s.SetDTLSRetransmissionInterval(100 * time.Millisecond)
	if s.dtls.retransmissionInterval == 0 ||
		s.dtls.retransmissionInterval != 100*time.Millisecond {
		t.Errorf("Failed to set DTLS retransmission interval")
	}

	s.SetDTLSRetransmissionInterval(1 * time.Second)
	if s.dtls.retransmissionInterval == 0 ||
		s.dtls.retransmissionInterval != 1*time.Second {
		t.Errorf("Failed to set DTLS retransmission interval")
	}
}

func TestSetDTLSEllipticCurves(t *testing.T) {
	s := SettingEngine{}

	if len(s.dtls.ellipticCurves) != 0 {
		t.Fatalf("SettingEngine defaults aren't as expected.")
	}

	s.SetDTLSEllipticCurves(elliptic.P256)
	if len(s.dtls.ellipticCurves) == 0 ||
		s.dtls.ellipticCurves[0] != elliptic.P256 {
		t.Errorf("Failed to set DTLS elliptic curves")
	}
}

func TestSetDTLSHandShakeTimeout(*testing.T) {
	s := SettingEngine{}

	s.SetDTLSConnectContextMaker(func() (context.Context, func()) {
		return context.WithTimeout(context.Background(), 60*time.Second)
	})
}

func TestSetSCTPMaxReceiverBufferSize(t *testing.T) {
	s := SettingEngine{}
	assert.Equal(t, uint32(0), s.sctp.maxReceiveBufferSize)

	expSize := uint32(4 * 1024 * 1024)
	s.SetSCTPMaxReceiveBufferSize(expSize)
	assert.Equal(t, expSize, s.sctp.maxReceiveBufferSize)
}

func TestSetICEBindingRequestHandler(t *testing.T) {
	seenICEControlled, seenICEControlledCancel := context.WithCancel(context.Background())
	seenICEControlling, seenICEControllingCancel := context.WithCancel(context.Background())

	s := SettingEngine{}
	s.SetICEBindingRequestHandler(func(m *stun.Message, _, _ ice.Candidate, _ *ice.CandidatePair) bool {
		for _, a := range m.Attributes {
			switch a.Type {
			case stun.AttrICEControlled:
				seenICEControlledCancel()
			case stun.AttrICEControlling:
				seenICEControllingCancel()
			default:
			}
		}

		return false
	})

	pcOffer, pcAnswer, err := NewAPI(WithSettingEngine(s)).newPair(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	<-seenICEControlled.Done()
	<-seenICEControlling.Done()
	closePairNow(t, pcOffer, pcAnswer)
}
