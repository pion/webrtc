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

	"github.com/pion/datachannel"
	"github.com/pion/dtls/v3/pkg/crypto/elliptic"
	"github.com/pion/dtls/v3/pkg/protocol/handshake"
	"github.com/pion/ice/v4"
	"github.com/pion/stun/v3"
	"github.com/pion/transport/v3/test"
	"github.com/stretchr/testify/assert"
)

func TestSetEphemeralUDPPortRange(t *testing.T) {
	settingEngine := SettingEngine{}

	if settingEngine.ephemeralUDP.PortMin != 0 ||
		settingEngine.ephemeralUDP.PortMax != 0 {
		t.Fatalf("SettingEngine defaults aren't as expected.")
	}

	// set bad ephemeral ports
	if err := settingEngine.SetEphemeralUDPPortRange(3000, 2999); err == nil {
		t.Fatalf("Setting engine should fail bad ephemeral ports.")
	}

	if err := settingEngine.SetEphemeralUDPPortRange(3000, 4000); err != nil {
		t.Fatalf("Setting engine failed valid port range: %s", err)
	}

	if settingEngine.ephemeralUDP.PortMin != 3000 ||
		settingEngine.ephemeralUDP.PortMax != 4000 {
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
	settingEngine := SettingEngine{}
	if settingEngine.candidates.NAT1To1IPs != nil {
		t.Errorf("Invalid default value")
	}
	if settingEngine.candidates.NAT1To1IPCandidateType != 0 {
		t.Errorf("Invalid default value")
	}

	ips := []string{"1.2.3.4"}
	typ := ICECandidateTypeHost
	settingEngine.SetNAT1To1IPs(ips, typ)
	if len(settingEngine.candidates.NAT1To1IPs) != 1 || settingEngine.candidates.NAT1To1IPs[0] != "1.2.3.4" {
		t.Fatalf("Failed to set NAT1To1IPs")
	}
	if settingEngine.candidates.NAT1To1IPCandidateType != typ {
		t.Fatalf("Failed to set NAT1To1IPCandidateType")
	}
}

func TestSetAnsweringDTLSRole(t *testing.T) {
	s := SettingEngine{}
	assert.Error(
		t,
		s.SetAnsweringDTLSRole(DTLSRoleAuto),
		"SetAnsweringDTLSRole can only be called with DTLSRoleClient or DTLSRoleServer",
	)
	assert.Error(
		t,
		s.SetAnsweringDTLSRole(DTLSRole(0)),
		"SetAnsweringDTLSRole can only be called with DTLSRoleClient or DTLSRoleServer",
	)
}

func TestSetReplayProtection(t *testing.T) {
	settingEngine := SettingEngine{}

	if settingEngine.replayProtection.DTLS != nil ||
		settingEngine.replayProtection.SRTP != nil ||
		settingEngine.replayProtection.SRTCP != nil {
		t.Fatalf("SettingEngine defaults aren't as expected.")
	}

	settingEngine.SetDTLSReplayProtectionWindow(128)
	settingEngine.SetSRTPReplayProtectionWindow(64)
	settingEngine.SetSRTCPReplayProtectionWindow(32)

	if settingEngine.replayProtection.DTLS == nil ||
		*settingEngine.replayProtection.DTLS != 128 {
		t.Errorf("Failed to set DTLS replay protection window")
	}
	if settingEngine.replayProtection.SRTP == nil ||
		*settingEngine.replayProtection.SRTP != 64 {
		t.Errorf("Failed to set SRTP replay protection window")
	}
	if settingEngine.replayProtection.SRTCP == nil ||
		*settingEngine.replayProtection.SRTCP != 32 {
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
		mediaEngine := &MediaEngine{}
		assert.NoError(t, mediaEngine.RegisterDefaultCodecs())

		api := NewAPI(WithMediaEngine(mediaEngine))

		offerer, answerer, err := api.newPair(Configuration{})
		assert.NoError(t, err)

		_, err = offerer.AddTransceiverFromKind(RTPCodecTypeVideo)
		assert.NoError(t, err)

		assert.NoError(t, signalPair(offerer, answerer))

		// Assert that the MediaEngine the user created isn't modified
		assert.False(t, mediaEngine.negotiatedVideo)
		assert.Empty(t, mediaEngine.negotiatedVideoCodecs)

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
		mediaEngine := &MediaEngine{}
		assert.NoError(t, mediaEngine.RegisterDefaultCodecs())

		s := SettingEngine{}
		s.DisableMediaEngineCopy(true)

		api := NewAPI(WithMediaEngine(mediaEngine), WithSettingEngine(s))

		offerer, answerer, err := api.newPair(Configuration{})
		assert.NoError(t, err)

		_, err = offerer.AddTransceiverFromKind(RTPCodecTypeVideo)
		assert.NoError(t, err)

		assert.NoError(t, signalPair(offerer, answerer))

		// Assert that the user MediaEngine was modified, so no copy happened
		assert.True(t, mediaEngine.negotiatedVideo)
		assert.NotEmpty(t, mediaEngine.negotiatedVideoCodecs)

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
	settingEngine := SettingEngine{}

	if settingEngine.dtls.retransmissionInterval != 0 {
		t.Fatalf("SettingEngine defaults aren't as expected.")
	}

	settingEngine.SetDTLSRetransmissionInterval(100 * time.Millisecond)
	if settingEngine.dtls.retransmissionInterval == 0 ||
		settingEngine.dtls.retransmissionInterval != 100*time.Millisecond {
		t.Errorf("Failed to set DTLS retransmission interval")
	}

	settingEngine.SetDTLSRetransmissionInterval(1 * time.Second)
	if settingEngine.dtls.retransmissionInterval == 0 ||
		settingEngine.dtls.retransmissionInterval != 1*time.Second {
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

func TestSetSCTPRTOMax(t *testing.T) {
	s := SettingEngine{}
	assert.Equal(t, time.Duration(0), s.sctp.rtoMax)

	expSize := time.Second
	s.SetSCTPRTOMax(expSize)
	assert.Equal(t, expSize, s.sctp.rtoMax)
}

func TestSetICEBindingRequestHandler(t *testing.T) {
	seenICEControlled, seenICEControlledCancel := context.WithCancel(context.Background())
	seenICEControlling, seenICEControllingCancel := context.WithCancel(context.Background())

	settingEngine := SettingEngine{}
	settingEngine.SetICEBindingRequestHandler(func(m *stun.Message, _, _ ice.Candidate, _ *ice.CandidatePair) bool {
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

	pcOffer, pcAnswer, err := NewAPI(WithSettingEngine(settingEngine)).newPair(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	<-seenICEControlled.Done()
	<-seenICEControlling.Done()
	closePairNow(t, pcOffer, pcAnswer)
}

func TestSetHooks(t *testing.T) {
	settingEngine := SettingEngine{}

	if settingEngine.dtls.clientHelloMessageHook != nil ||
		settingEngine.dtls.serverHelloMessageHook != nil ||
		settingEngine.dtls.certificateRequestMessageHook != nil {
		t.Fatalf("SettingEngine defaults aren't as expected.")
	}

	settingEngine.SetDTLSClientHelloMessageHook(func(msg handshake.MessageClientHello) handshake.Message {
		return &msg
	})
	settingEngine.SetDTLSServerHelloMessageHook(func(msg handshake.MessageServerHello) handshake.Message {
		return &msg
	})
	settingEngine.SetDTLSCertificateRequestMessageHook(func(msg handshake.MessageCertificateRequest) handshake.Message {
		return &msg
	})

	if settingEngine.dtls.clientHelloMessageHook == nil {
		t.Errorf("Failed to set DTLS Client Hello Hook")
	}
	if settingEngine.dtls.serverHelloMessageHook == nil {
		t.Errorf("Failed to set DTLS Server Hello Hook")
	}
	if settingEngine.dtls.certificateRequestMessageHook == nil {
		t.Errorf("Failed to set DTLS Certificate Request Hook")
	}
}

func TestSetFireOnTrackBeforeFirstRTP(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	settingEngine := SettingEngine{}
	settingEngine.SetFireOnTrackBeforeFirstRTP(true)

	mediaEngineOne := &MediaEngine{}
	assert.NoError(t, mediaEngineOne.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:     "video/VP8",
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "",
			RTCPFeedback: nil,
		},
		PayloadType: 100,
	}, RTPCodecTypeVideo))

	mediaEngineTwo := &MediaEngine{}
	assert.NoError(t, mediaEngineTwo.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:     "video/VP8",
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "",
			RTCPFeedback: nil,
		},
		PayloadType: 200,
	}, RTPCodecTypeVideo))

	offerer, err := NewAPI(WithMediaEngine(mediaEngineOne), WithSettingEngine(settingEngine)).NewPeerConnection(
		Configuration{},
	)
	assert.NoError(t, err)

	answerer, err := NewAPI(WithMediaEngine(mediaEngineTwo)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = offerer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	_, err = answerer.AddTrack(track)
	assert.NoError(t, err)

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	offerer.OnTrack(func(track *TrackRemote, _ *RTPReceiver) {
		_, _, err = track.Read(make([]byte, 1500))
		assert.NoError(t, err)
		assert.Equal(t, track.PayloadType(), PayloadType(100))
		assert.Equal(t, track.Codec().RTPCodecCapability.MimeType, "video/VP8")

		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(offerer, answerer))

	sendVideoUntilDone(t, onTrackFired.Done(), []*TrackLocalStaticSample{track})

	closePairNow(t, offerer, answerer)
}

func TestDisableCloseByDTLS(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	s := SettingEngine{}
	s.DisableCloseByDTLS(true)

	offer, answer, err := NewAPI(WithSettingEngine(s)).newPair(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, signalPair(offer, answer))

	untilConnectionState(PeerConnectionStateConnected, offer, answer).Wait()
	assert.NoError(t, answer.Close())

	time.Sleep(time.Second)
	assert.True(t, offer.ConnectionState() == PeerConnectionStateConnected)
	assert.NoError(t, offer.Close())
}

func TestEnableDataChannelBlockWrite(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	s := SettingEngine{}
	s.DetachDataChannels()
	s.EnableDataChannelBlockWrite(true)
	s.SetSCTPMaxReceiveBufferSize(1500)

	offer, answer, err := NewAPI(WithSettingEngine(s)).newPair(Configuration{})
	assert.NoError(t, err)

	dc, err := offer.CreateDataChannel("data", nil)
	assert.NoError(t, err)
	detachChan := make(chan datachannel.ReadWriteCloserDeadliner, 1)
	dc.OnOpen(func() {
		detached, err1 := dc.DetachWithDeadline()
		assert.NoError(t, err1)
		detachChan <- detached
	})

	assert.NoError(t, signalPair(offer, answer))
	untilConnectionState(PeerConnectionStateConnected, offer, answer).Wait()

	// write should block and return deadline exceeded since the receiver is not reading
	// and the buffer size is 1500 bytes
	rawDC := <-detachChan
	assert.NoError(t, rawDC.SetWriteDeadline(time.Now().Add(time.Second)))
	buf := make([]byte, 1000)
	for i := 0; i < 10; i++ {
		_, err = rawDC.Write(buf)
		if err != nil {
			break
		}
	}
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	closePairNow(t, offer, answer)
}
