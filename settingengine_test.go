// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"bytes"
	"context"
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/pion/datachannel"
	"github.com/pion/dtls/v3"
	"github.com/pion/dtls/v3/pkg/crypto/elliptic"
	"github.com/pion/dtls/v3/pkg/protocol/handshake"
	"github.com/pion/ice/v4"
	"github.com/pion/stun/v3"
	"github.com/pion/transport/v3/test"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/proxy"
)

func TestSetEphemeralUDPPortRange(t *testing.T) {
	settingEngine := SettingEngine{}
	assert.Equal(t, uint16(0), settingEngine.ephemeralUDP.PortMin)
	assert.Equal(t, uint16(0), settingEngine.ephemeralUDP.PortMax)

	// set bad ephemeral ports
	assert.Error(
		t, settingEngine.SetEphemeralUDPPortRange(3000, 2999),
		"Setting engine should fail bad ephemeral ports",
	)

	assert.NoError(t, settingEngine.SetEphemeralUDPPortRange(3000, 4000))
	assert.Equal(t, uint16(3000), settingEngine.ephemeralUDP.PortMin)
	assert.Equal(t, uint16(4000), settingEngine.ephemeralUDP.PortMax)
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

func TestICERenomination(t *testing.T) {
	t.Run("EnableWithDefaultGenerator", func(t *testing.T) {
		s := SettingEngine{}
		assert.NoError(t, s.SetICERenomination())

		assert.True(t, s.renomination.enabled)
		assert.NotNil(t, s.renomination.generator)
		assert.Equal(t, uint32(1), s.renomination.generator())
		assert.Equal(t, uint32(2), s.renomination.generator())
	})

	t.Run("AutomaticRenominationUsesExistingGenerator", func(t *testing.T) {
		var calls uint32
		settings := SettingEngine{}
		customGen := func() uint32 {
			calls++

			return 100 + calls
		}

		interval := 2 * time.Second
		assert.NoError(t, settings.SetICERenomination(
			WithRenominationGenerator(customGen),
			WithRenominationInterval(interval),
		))

		assert.True(t, settings.renomination.enabled)
		assert.True(t, settings.renomination.automatic)
		if assert.NotNil(t, settings.renomination.automaticInterval) {
			assert.Equal(t, interval, *settings.renomination.automaticInterval)
		}
		assert.Equal(t, uint32(101), settings.renomination.generator())
	})

	t.Run("AutomaticRenominationEnablesGenerator", func(t *testing.T) {
		s := SettingEngine{}
		assert.NoError(t, s.SetICERenomination())

		assert.True(t, s.renomination.enabled)
		assert.True(t, s.renomination.automatic)
		assert.Nil(t, s.renomination.automaticInterval)
		assert.NotNil(t, s.renomination.generator)
	})

	t.Run("InvalidInterval", func(t *testing.T) {
		s := SettingEngine{}
		assert.ErrorIs(t, s.SetICERenomination(WithRenominationInterval(0)), errInvalidRenominationInterval)
		assert.ErrorIs(t, s.SetICERenomination(WithRenominationInterval(-1*time.Second)), errInvalidRenominationInterval)
	})
}

func TestDetachDataChannels(t *testing.T) {
	s := SettingEngine{}
	assert.False(t, s.detach.DataChannels)

	s.DetachDataChannels()
	assert.True(t, s.detach.DataChannels, "Failed to enable detached data channels.")
}

func TestSetNAT1To1IPs(t *testing.T) {
	settingEngine := SettingEngine{}
	assert.Nil(t, settingEngine.candidates.NAT1To1IPs)
	assert.Equal(t, ICECandidateType(0), settingEngine.candidates.NAT1To1IPCandidateType)

	ips := []string{"1.2.3.4"}
	typ := ICECandidateTypeHost
	settingEngine.SetNAT1To1IPs(ips, typ)
	assert.Equal(t, ips, settingEngine.candidates.NAT1To1IPs, "Failed to set NAT1To1IPs")
	assert.Equal(t, typ, settingEngine.candidates.NAT1To1IPCandidateType, "Failed to set NAT1To1IPCandidateType")
}

func TestSettingEngine_SetICEAddressRewriteRules_EmptyClears(t *testing.T) {
	se := SettingEngine{}
	assert.Nil(t, se.candidates.addressRewriteRules)

	assert.NoError(t, se.SetICEAddressRewriteRules(ICEAddressRewriteRule{
		External:        []string{"198.51.100.1"},
		AsCandidateType: ICECandidateTypeHost,
		Mode:            ICEAddressRewriteReplace,
	}))
	assert.NotNil(t, se.candidates.addressRewriteRules)
	assert.Len(t, se.candidates.addressRewriteRules, 1)

	se.SetNAT1To1IPs([]string{"203.0.113.1"}, ICECandidateTypeHost)
	assert.NoError(t, se.SetICEAddressRewriteRules())
	assert.Nil(t, se.candidates.addressRewriteRules)

	assert.ErrorIs(t, se.SetICEAddressRewriteRules(ICEAddressRewriteRule{
		External:        []string{"198.51.100.2"},
		AsCandidateType: ICECandidateTypeHost,
		Mode:            ICEAddressRewriteReplace,
	}), errAddressRewriteWithNAT1To1)
}

// ExampleSettingEngine_SetICEAddressRewriteRules_replaceHost demonstrates
// replacing host candidates with a fixed public address using the rewrite API.
func ExampleSettingEngine_SetICEAddressRewriteRules_replaceHost() {
	var se SettingEngine

	_ = se.SetICEAddressRewriteRules(
		ICEAddressRewriteRule{
			External:        []string{"198.51.100.1"},
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteReplace,
		},
	)
}

// ExampleSettingEngine_SetICEAddressRewriteRules_appendSrflx demonstrates
// appending a server reflexive candidate that advertises a public address while
// still keeping the host candidate.
func ExampleSettingEngine_SetICEAddressRewriteRules_appendSrflx() {
	var se SettingEngine

	_ = se.SetICEAddressRewriteRules(
		ICEAddressRewriteRule{
			External:        []string{"198.51.100.2"},
			AsCandidateType: ICECandidateTypeSrflx,
			Mode:            ICEAddressRewriteAppend,
		},
	)
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

	assert.Nil(t, settingEngine.replayProtection.DTLS)
	assert.Nil(t, settingEngine.replayProtection.SRTP)
	assert.Nil(t, settingEngine.replayProtection.SRTCP)

	settingEngine.SetDTLSReplayProtectionWindow(128)
	settingEngine.SetSRTPReplayProtectionWindow(64)
	settingEngine.SetSRTCPReplayProtectionWindow(32)

	assert.NotNil(
		t, settingEngine.replayProtection.DTLS,
		"DTLS replay protection window should not be nil",
	)
	assert.Equal(
		t, uint(128), *settingEngine.replayProtection.DTLS,
		"Failed to set DTLS replay protection window",
	)

	assert.NotNil(
		t, settingEngine.replayProtection.SRTP,
		"SRTP replay protection window should not be nil",
	)
	assert.Equal(
		t, uint(64), *settingEngine.replayProtection.SRTP,
		"Failed to set SRTP replay protection window",
	)
	assert.NotNil(
		t, settingEngine.replayProtection.SRTCP,
		"SRTCP replay protection window should not be nil",
	)
	assert.Equal(
		t, uint(32), *settingEngine.replayProtection.SRTCP,
		"Failed to set SRTCP replay protection window",
	)
}

func TestSettingEngine_SetICETCP(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{})
	assert.NoError(t, err)

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

	assert.Equal(t, time.Duration(0), settingEngine.dtls.retransmissionInterval)

	settingEngine.SetDTLSRetransmissionInterval(100 * time.Millisecond)
	assert.Equal(
		t, 100*time.Millisecond, settingEngine.dtls.retransmissionInterval,
		"Failed to set DTLS retransmission interval",
	)

	settingEngine.SetDTLSRetransmissionInterval(1 * time.Second)
	assert.Equal(
		t, 1*time.Second, settingEngine.dtls.retransmissionInterval,
		"Failed to set DTLS retransmission interval",
	)
}

func TestSetDTLSEllipticCurves(t *testing.T) {
	s := SettingEngine{}
	assert.Empty(t, s.dtls.ellipticCurves)

	s.SetDTLSEllipticCurves(elliptic.P256)
	assert.NotEmpty(t, s.dtls.ellipticCurves, "Failed to set DTLS elliptic curves")
	assert.Equal(t, elliptic.P256, s.dtls.ellipticCurves[0])
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

	assert.Nil(t, settingEngine.dtls.clientHelloMessageHook)
	assert.Nil(t, settingEngine.dtls.serverHelloMessageHook)
	assert.Nil(t, settingEngine.dtls.certificateRequestMessageHook)

	settingEngine.SetDTLSClientHelloMessageHook(func(msg handshake.MessageClientHello) handshake.Message {
		return &msg
	})
	settingEngine.SetDTLSServerHelloMessageHook(func(msg handshake.MessageServerHello) handshake.Message {
		return &msg
	})
	settingEngine.SetDTLSCertificateRequestMessageHook(func(msg handshake.MessageCertificateRequest) handshake.Message {
		return &msg
	})

	assert.NotNil(
		t, settingEngine.dtls.clientHelloMessageHook,
		"Failed to set DTLS Client Hello Hook",
	)
	assert.NotNil(
		t, settingEngine.dtls.serverHelloMessageHook,
		"Failed to set DTLS Server Hello Hook",
	)
	assert.NotNil(
		t, settingEngine.dtls.certificateRequestMessageHook,
		"Failed to set DTLS Certificate Request Hook",
	)
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

func TestSettingEngine_getReceiveMTU_Custom(t *testing.T) {
	var se SettingEngine
	se.SetReceiveMTU(1234)

	got := se.getReceiveMTU()
	assert.Equal(t, uint(1234), got)
}

func TestSettingEngine_ICEAcceptanceAndSTUNSetters(t *testing.T) {
	var se SettingEngine

	host := 10 * time.Millisecond
	srflx := 20 * time.Millisecond
	prflx := 30 * time.Millisecond
	relay := 40 * time.Millisecond
	stun := 50 * time.Millisecond

	se.SetHostAcceptanceMinWait(host)
	se.SetSrflxAcceptanceMinWait(srflx)
	se.SetPrflxAcceptanceMinWait(prflx)
	se.SetRelayAcceptanceMinWait(relay)
	se.SetSTUNGatherTimeout(stun)

	assert.NotNil(t, se.timeout.ICEHostAcceptanceMinWait)
	assert.NotNil(t, se.timeout.ICESrflxAcceptanceMinWait)
	assert.NotNil(t, se.timeout.ICEPrflxAcceptanceMinWait)
	assert.NotNil(t, se.timeout.ICERelayAcceptanceMinWait)
	assert.NotNil(t, se.timeout.ICESTUNGatherTimeout)

	assert.Equal(t, host, *se.timeout.ICEHostAcceptanceMinWait)
	assert.Equal(t, srflx, *se.timeout.ICESrflxAcceptanceMinWait)
	assert.Equal(t, prflx, *se.timeout.ICEPrflxAcceptanceMinWait)
	assert.Equal(t, relay, *se.timeout.ICERelayAcceptanceMinWait)
	assert.Equal(t, stun, *se.timeout.ICESTUNGatherTimeout)
}

func TestSettingEngine_CandidateFiltersAndNetworkTypes(t *testing.T) {
	var se SettingEngine

	nts := []NetworkType{NetworkTypeUDP4, NetworkTypeUDP6}
	se.SetNetworkTypes(nts)
	assert.Equal(t, nts, se.candidates.ICENetworkTypes)

	ifFilter := func(name string) bool { return name == "eth0" }
	ipFilter := func(ip net.IP) bool { return ip.IsLoopback() }

	se.SetInterfaceFilter(ifFilter)
	se.SetIPFilter(ipFilter)
	se.SetIncludeLoopbackCandidate(true)

	assert.NotNil(t, se.candidates.InterfaceFilter)
	assert.NotNil(t, se.candidates.IPFilter)
	assert.True(t, se.candidates.InterfaceFilter("eth0"))
	assert.False(t, se.candidates.InterfaceFilter("wlan0"))
	assert.True(t, se.candidates.IPFilter(net.IPv4(127, 0, 0, 1)))
	assert.True(t, se.candidates.IncludeLoopbackCandidate)
}

func TestSettingEngine_MDNSAndCredentialsAndFingerprint(t *testing.T) {
	var se SettingEngine

	se.SetMulticastDNSHostName("host.local.")
	se.SetICECredentials("ufrag123", "pwd456")
	se.DisableCertificateFingerprintVerification(true)

	assert.Equal(t, "host.local.", se.candidates.MulticastDNSHostName)
	assert.Equal(t, "ufrag123", se.candidates.UsernameFragment)
	assert.Equal(t, "pwd456", se.candidates.Password)
	assert.True(t, se.disableCertificateFingerprintVerification)
}

func TestSettingEngine_UDPMuxProxyBindingAndTCPFlags(t *testing.T) {
	var se SettingEngine

	var mux ice.UDPMux
	se.SetICEUDPMux(mux)
	assert.Equal(t, mux, se.iceUDPMux)

	se.SetICEProxyDialer(proxy.Direct)
	assert.Equal(t, proxy.Direct, se.iceProxyDialer)

	var maxReq uint16 = 77
	se.SetICEMaxBindingRequests(maxReq)
	assert.NotNil(t, se.iceMaxBindingRequests)
	assert.Equal(t, maxReq, *se.iceMaxBindingRequests)

	se.DisableActiveTCP(true)
	assert.True(t, se.iceDisableActiveTCP)
}

func TestSettingEngine_MediaEngineAndMTUFlags(t *testing.T) {
	var se SettingEngine

	se.DisableMediaEngineMultipleCodecs(true)
	assert.True(t, se.disableMediaEngineMultipleCodecs)

	se.SetReceiveMTU(1337)
	assert.Equal(t, uint(1337), se.receiveMTU)
}

func TestSettingEngine_DTLSSetters(t *testing.T) {
	var se SettingEngine

	se.SetDTLSInsecureSkipHelloVerify(true)
	se.SetDTLSDisableInsecureSkipVerify(true)
	se.SetDTLSExtendedMasterSecret(dtls.RequireExtendedMasterSecret)

	auth := dtls.RequireAnyClientCert
	se.SetDTLSClientAuth(auth)

	clientCAs := x509.NewCertPool()
	rootCAs := x509.NewCertPool()
	var keyBuf bytes.Buffer

	se.SetDTLSClientCAs(clientCAs)
	se.SetDTLSRootCAs(rootCAs)
	se.SetDTLSKeyLogWriter(&keyBuf)
	se.SetDTLSCipherSuites(dtls.TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8, dtls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256)

	called := false
	se.SetDTLSCustomerCipherSuites(func() []dtls.CipherSuite {
		called = true

		return nil
	})

	assert.True(t, se.dtls.insecureSkipHelloVerify)
	assert.True(t, se.dtls.disableInsecureSkipVerify)
	assert.Equal(t, dtls.RequireExtendedMasterSecret, se.dtls.extendedMasterSecret)
	assert.NotNil(t, se.dtls.clientAuth)
	assert.Equal(t, auth, *se.dtls.clientAuth)
	assert.Equal(t, clientCAs, se.dtls.clientCAs)
	assert.Equal(t, rootCAs, se.dtls.rootCAs)
	_, _ = se.dtls.keyLogWriter.Write([]byte("test"))
	assert.NotZero(t, keyBuf.Len())
	assert.Equal(t, []dtls.CipherSuiteID{
		dtls.TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8,
		dtls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	}, se.dtls.cipherSuites)
	_ = se.dtls.customCipherSuites()
	assert.True(t, called)
}

func TestSettingEngine_SCTPSetters(t *testing.T) {
	var se SettingEngine

	se.EnableSCTPZeroChecksum(true)
	se.SetSCTPMinCwnd(11)
	se.SetSCTPFastRtxWnd(22)
	se.SetSCTPCwndCAStep(33)

	assert.True(t, se.sctp.enableZeroChecksum)
	assert.Equal(t, uint32(11), se.sctp.minCwnd)
	assert.Equal(t, uint32(22), se.sctp.fastRtxWnd)
	assert.Equal(t, uint32(33), se.sctp.cwndCAStep)
}

func TestSettingEngine_HandleUndeclaredSSRCWithoutAnswer(t *testing.T) {
	var se SettingEngine
	se.SetHandleUndeclaredSSRCWithoutAnswer(true)
	assert.True(t, se.handleUndeclaredSSRCWithoutAnswer)
}
