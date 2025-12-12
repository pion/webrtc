// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/logging"
	"github.com/pion/stun/v3"
	"github.com/pion/transport/v3/test"
	"github.com/pion/transport/v3/vnet"
	"github.com/pion/turn/v4"
	"github.com/stretchr/testify/assert"
)

func TestNewICEGatherer_Success(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	opts := ICEGatherOptions{
		ICEServers: []ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}

	gatherer, err := NewAPI().NewICEGatherer(opts)
	assert.NoError(t, err)
	assert.Equal(t, ICEGathererStateNew, gatherer.State())

	gatherFinished := make(chan struct{})
	gatherer.OnLocalCandidate(func(i *ICECandidate) {
		if i == nil {
			close(gatherFinished)
		}
	})

	assert.NoError(t, gatherer.Gather())

	<-gatherFinished

	params, err := gatherer.GetLocalParameters()
	assert.NoError(t, err)

	assert.NotEmpty(t, params.UsernameFragment, "Empty local username frag")
	assert.NotEmpty(t, params.Password, "Empty local password")

	candidates, err := gatherer.GetLocalCandidates()
	assert.NoError(t, err)
	assert.NotEmpty(t, candidates, "No candidates gathered")

	assert.NoError(t, gatherer.Close())
}

func TestICEGather_mDNSCandidateGathering(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	s := SettingEngine{}
	s.SetICEMulticastDNSMode(ice.MulticastDNSModeQueryAndGather)

	gatherer, err := NewAPI(WithSettingEngine(s)).NewICEGatherer(ICEGatherOptions{})
	assert.NoError(t, err)

	gotMulticastDNSCandidate, resolveFunc := context.WithCancel(context.Background())
	gatherer.OnLocalCandidate(func(c *ICECandidate) {
		if c != nil && strings.HasSuffix(c.Address, ".local") {
			resolveFunc()
		}
	})

	assert.NoError(t, gatherer.Gather())

	<-gotMulticastDNSCandidate.Done()
	assert.NoError(t, gatherer.Close())
}

func TestICEGatherer_InvalidMDNSHostName(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	se := SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeQueryAndGather)
	se.SetMulticastDNSHostName("bad..local")

	gatherer, err := NewAPI(WithSettingEngine(se)).NewICEGatherer(ICEGatherOptions{})
	assert.NoError(t, err)

	err = gatherer.Gather()
	assert.ErrorIs(t, err, ice.ErrInvalidMulticastDNSHostName)
}

func TestLegacyNAT1To1AddressRewriteRules(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, legacyNAT1To1AddressRewriteRules(nil, ice.CandidateTypeHost))
	})

	t.Run("mapping and catch-all", func(t *testing.T) {
		ips := []string{
			"1.2.3.4/10.0.0.1",
			"5.6.7.8/10.0.0.2",
			"9.9.9.9",
		}
		rules := legacyNAT1To1AddressRewriteRules(ips, ice.CandidateTypeServerReflexive)

		assert.Equal(t, []ice.AddressRewriteRule{
			{
				External:        []string{"1.2.3.4"},
				Local:           "10.0.0.1",
				AsCandidateType: ice.CandidateTypeServerReflexive,
			},
			{
				External:        []string{"5.6.7.8"},
				Local:           "10.0.0.2",
				AsCandidateType: ice.CandidateTypeServerReflexive,
			},
			{
				External: []string{
					"1.2.3.4",
					"5.6.7.8",
					"9.9.9.9",
				},
				AsCandidateType: ice.CandidateTypeServerReflexive,
			},
		}, rules)
	})
}

func TestLegacyNAT1To1AddressRewriteRulesVNet(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		externalIP = "203.0.113.1"
		localIP    = "10.0.0.1"
	)

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	assert.NoError(t, err)

	nw, err := vnet.NewNet(&vnet.NetConfig{
		StaticIP: localIP,
	})
	assert.NoError(t, err)
	assert.NoError(t, router.AddNet(nw))
	assert.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	run := func(candidateType ICECandidateType) []ICECandidate {
		se := SettingEngine{}
		se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
		se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
		se.SetNet(nw)
		se.SetNAT1To1IPs([]string{fmt.Sprintf("%s/%s", externalIP, localIP)}, candidateType)

		gatherer, err := NewAPI(WithSettingEngine(se)).NewICEGatherer(ICEGatherOptions{})
		assert.NoError(t, err)
		defer func() {
			assert.NoError(t, gatherer.Close())
		}()

		done := make(chan struct{})
		var candidates []ICECandidate
		gatherer.OnLocalCandidate(func(c *ICECandidate) {
			if c == nil {
				close(done)
			} else {
				candidates = append(candidates, *c)
			}
		})

		assert.NoError(t, gatherer.Gather())
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			assert.Fail(t, "gather did not complete")
		}

		return candidates
	}

	t.Run("HostReplace", func(t *testing.T) {
		candidates := run(ICECandidateTypeHost)
		assert.NotEmpty(t, candidates)

		var hostAddrs []string
		for _, c := range candidates {
			if c.Typ == ICECandidateTypeHost {
				hostAddrs = append(hostAddrs, c.Address)
			}
		}

		assert.NotEmpty(t, hostAddrs, "expected host candidates")
		assert.Subset(t, hostAddrs, []string{externalIP})
		for _, addr := range hostAddrs {
			assert.NotEqual(t, localIP, addr)
		}
	})

	t.Run("SrflxAppend", func(t *testing.T) {
		candidates := run(ICECandidateTypeSrflx)
		assert.NotEmpty(t, candidates)

		var hostAddrs []string
		var srflx ICECandidate
		var haveSrflx bool
		for _, c := range candidates {
			switch c.Typ {
			case ICECandidateTypeHost:
				hostAddrs = append(hostAddrs, c.Address)
			case ICECandidateTypeSrflx:
				srflx = c
				haveSrflx = true
			default:
			}
		}

		assert.NotEmpty(t, hostAddrs, "expected host candidates")
		assert.Contains(t, hostAddrs, localIP)
		assert.True(t, haveSrflx, "expected srflx candidate")
		assert.Equal(t, externalIP, srflx.Address)
	})
}

func TestICEGatherer_StaticLocalCredentialsVNet(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	parseCreds := func(sdp string) (string, string) {
		var ufrag, pwd string
		for _, l := range strings.Split(sdp, "\n") {
			l = strings.TrimSpace(l)
			if after, ok := strings.CutPrefix(l, "a=ice-ufrag:"); ok {
				ufrag = after
			} else if after, ok := strings.CutPrefix(l, "a=ice-pwd:"); ok {
				pwd = after
			}
		}

		return ufrag, pwd
	}

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	assert.NoError(t, err)

	offerNet, err := vnet.NewNet(&vnet.NetConfig{StaticIPs: []string{"10.0.0.2"}})
	assert.NoError(t, err)
	answerNet, err := vnet.NewNet(&vnet.NetConfig{StaticIPs: []string{"10.0.0.3"}})
	assert.NoError(t, err)

	assert.NoError(t, router.AddNet(offerNet))
	assert.NoError(t, router.AddNet(answerNet))
	assert.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	buildSE := func(n *vnet.Net, ufrag, pwd string) SettingEngine {
		se := SettingEngine{}
		se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
		se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
		se.SetNet(n)
		se.SetICECredentials(ufrag, pwd)

		return se
	}

	const (
		offerUfrag  = "offerufrag123"
		offerPwd    = "offerpassword123456"
		answerUfrag = "answerufrag123"
		answerPwd   = "answerpassword123456"
	)

	pcOffer, err := NewAPI(WithSettingEngine(buildSE(offerNet, offerUfrag, offerPwd))).NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	pcAnswer, err := NewAPI(
		WithSettingEngine(buildSE(answerNet, answerUfrag, answerPwd)),
	).NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	defer closePairNow(t, pcOffer, pcAnswer)

	connected := untilConnectionState(PeerConnectionStateConnected, pcOffer, pcAnswer)
	assert.NoError(t, signalPair(pcOffer, pcAnswer))
	connected.Wait()

	gotUfrag, gotPwd := parseCreds(pcOffer.LocalDescription().SDP)
	assert.Equal(t, offerUfrag, gotUfrag)
	assert.Equal(t, offerPwd, gotPwd)

	gotUfrag, gotPwd = parseCreds(pcAnswer.LocalDescription().SDP)
	assert.Equal(t, answerUfrag, gotUfrag)
	assert.Equal(t, answerPwd, gotPwd)
}

func TestICEGatherer_AlreadyClosed(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	opts := ICEGatherOptions{
		ICEServers: []ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}

	t.Run("Gather", func(t *testing.T) {
		gatherer, err := NewAPI().NewICEGatherer(opts)
		assert.NoError(t, err)

		err = gatherer.createAgent()
		assert.NoError(t, err)

		err = gatherer.Close()
		assert.NoError(t, err)

		err = gatherer.Gather()
		assert.ErrorIs(t, err, errICEAgentNotExist)
	})

	t.Run("GetLocalParameters", func(t *testing.T) {
		gatherer, err := NewAPI().NewICEGatherer(opts)
		assert.NoError(t, err)

		err = gatherer.createAgent()
		assert.NoError(t, err)

		err = gatherer.Close()
		assert.NoError(t, err)

		_, err = gatherer.GetLocalParameters()
		assert.ErrorIs(t, err, errICEAgentNotExist)
	})

	t.Run("GetLocalCandidates", func(t *testing.T) {
		gatherer, err := NewAPI().NewICEGatherer(opts)
		assert.NoError(t, err)

		err = gatherer.createAgent()
		assert.NoError(t, err)

		err = gatherer.Close()
		assert.NoError(t, err)

		_, err = gatherer.GetLocalCandidates()
		assert.ErrorIs(t, err, errICEAgentNotExist)
	})
}

func TestICEGatherer_MaxBindingRequests(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const maxReq uint16 = 2

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "1.2.3.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	if !assert.NoError(t, err) {
		return
	}

	offerNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{"1.2.3.4"},
	})
	if !assert.NoError(t, err) {
		return
	}

	answerNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{"1.2.3.5"},
	})
	if !assert.NoError(t, err) {
		return
	}

	if !assert.NoError(t, router.AddNet(offerNet)) {
		return
	}
	if !assert.NoError(t, router.AddNet(answerNet)) {
		return
	}

	answerIP := net.ParseIP("1.2.3.5")
	router.AddChunkFilter(func(c vnet.Chunk) bool {
		if addr, ok := c.SourceAddr().(*net.UDPAddr); ok {
			// drop all packets originating from the answerer so the offerer
			// never receives binding responses.
			return !addr.IP.Equal(answerIP)
		}

		return true
	})

	if !assert.NoError(t, router.Start()) {
		return
	}
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	offerS := SettingEngine{}
	offerS.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	offerS.SetICEMaxBindingRequests(maxReq)
	offerS.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	offerS.SetNet(offerNet)

	var bindingRequests atomic.Uint32
	firstRequest := make(chan struct{})
	answerSE := SettingEngine{}
	answerSE.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	answerSE.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	answerSE.SetNet(answerNet)
	answerSE.SetICEBindingRequestHandler(func(_ *stun.Message, _, _ ice.Candidate, _ *ice.CandidatePair) bool {
		bindingRequests.Add(1)
		select {
		case firstRequest <- struct{}{}:
		default:
		}

		return false
	})

	pcOffer, err := NewAPI(WithSettingEngine(offerS)).NewPeerConnection(Configuration{})
	if !assert.NoError(t, err) {
		return
	}
	pcAnswer, err := NewAPI(WithSettingEngine(answerSE)).NewPeerConnection(Configuration{})
	if !assert.NoError(t, err) {
		return
	}
	defer closePairNow(t, pcOffer, pcAnswer)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	select {
	case <-firstRequest:
	case <-time.After(2 * time.Second):
		assert.Fail(t, "did not receive any binding request")
	}

	expected := uint32(maxReq) + 1
	finalCount := func() uint32 {
		last := bindingRequests.Load()
		deadline := time.Now().Add(5 * time.Second)

		for time.Now().Before(deadline) {
			time.Sleep(150 * time.Millisecond)
			next := bindingRequests.Load()
			if next == last && next >= expected {
				return next
			}
			last = next
		}

		return bindingRequests.Load()
	}()

	assert.Equal(t, expected, finalCount, "max binding requests should limit retransmits")
}

func TestICEGatherer_DisableActiveTCP(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	tests := []struct {
		name            string
		disableActive   bool
		expectConnected bool
	}{
		{
			name:            "ActiveTCPEnabled",
			disableActive:   false,
			expectConnected: true,
		},
		{
			name:            "ActiveTCPDisabled",
			disableActive:   true,
			expectConnected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
			if err != nil || listener == nil {
				t.Skip("tcp listener unavailable in this environment")
			}
			defer func() {
				assert.NoError(t, listener.Close())
			}()

			accepted := make(chan struct{})
			go func() {
				conn, acceptErr := listener.Accept()
				if acceptErr == nil {
					if closeErr := conn.Close(); closeErr != nil {
						t.Logf("close accepted conn: %v", closeErr)
					}
				}
				close(accepted)
			}()

			se := SettingEngine{}
			se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
			se.SetNetworkTypes([]NetworkType{NetworkTypeTCP4})
			se.SetICETimeouts(time.Second, 2*time.Second, 500*time.Millisecond)
			se.SetIncludeLoopbackCandidate(true)
			se.DisableActiveTCP(tt.disableActive)

			gatherer, err := NewAPI(WithSettingEngine(se)).NewICEGatherer(ICEGatherOptions{})
			assert.NoError(t, err)
			defer func() {
				assert.NoError(t, gatherer.Close())
			}()

			assert.NoError(t, gatherer.createAgent())

			agent := gatherer.getAgent()
			if !assert.NotNil(t, agent) {
				return
			}

			addr, ok := listener.Addr().(*net.TCPAddr)
			if !assert.True(t, ok) {
				return
			}

			c, err := ice.NewCandidateHost(&ice.CandidateHostConfig{
				Network:   "tcp4",
				Address:   addr.IP.String(),
				Port:      addr.Port,
				Component: ice.ComponentRTP,
				TCPType:   ice.TCPTypePassive,
			})
			assert.NoError(t, err)
			assert.NoError(t, agent.AddRemoteCandidate(c))

			select {
			case <-accepted:
				assert.False(t, tt.disableActive, "active TCP dialed despite being disabled")
			case <-time.After(3 * time.Second):
				assert.True(t, tt.disableActive, "expected active TCP dial when enabled")
			}
		})
	}
}

func TestICEGatherer_HostAcceptanceMinWait(t *testing.T) {
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const wait = 500 * time.Millisecond

	pcOffer, pcAnswer, wan := createVNetPair(t, nil)
	defer func() {
		assert.NoError(t, wan.Stop())
		closePairNow(t, pcOffer, pcAnswer)
	}()

	pcOffer.api.settingEngine.timeout.ICEHostAcceptanceMinWait = func() *time.Duration {
		d := wait

		return &d
	}()

	start := time.Now()
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	connected := untilConnectionState(PeerConnectionStateConnected, pcOffer, pcAnswer)
	connected.Wait()

	assert.GreaterOrEqual(t, time.Since(start), wait)
}

func TestICEGatherer_SrflxAcceptanceMinWait(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 40)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		stunIP               = "1.2.3.4"
		stunPort             = 3478
		defaultSrflxMinWait  = 500 * time.Millisecond
		offerExternalIP      = "1.2.3.10"
		offerLocalIP         = "10.0.0.1"
		answerExternalIP     = "1.2.3.11"
		answerLocalIP        = "10.0.1.1"
		externalRouterSubnet = "1.2.3.0/24"
	)
	wait := 900 * time.Millisecond

	loggerFactory := logging.NewDefaultLoggerFactory()

	wan, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          externalRouterSubnet,
		LoggerFactory: loggerFactory,
	})
	assert.NoError(t, err)

	stunNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIP: stunIP,
	})
	assert.NoError(t, err)
	assert.NoError(t, wan.AddNet(stunNet))

	offerLAN, err := vnet.NewRouter(&vnet.RouterConfig{
		StaticIPs: []string{fmt.Sprintf("%s/%s", offerExternalIP, offerLocalIP)},
		CIDR:      "10.0.0.0/24",
		NATType: &vnet.NATType{
			Mode: vnet.NATModeNAT1To1,
		},
		LoggerFactory: loggerFactory,
	})
	assert.NoError(t, err)

	offerNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{offerLocalIP},
	})
	assert.NoError(t, err)
	assert.NoError(t, offerLAN.AddNet(offerNet))
	assert.NoError(t, wan.AddRouter(offerLAN))

	answerLAN, err := vnet.NewRouter(&vnet.RouterConfig{
		StaticIPs: []string{fmt.Sprintf("%s/%s", answerExternalIP, answerLocalIP)},
		CIDR:      "10.0.1.0/24",
		NATType: &vnet.NATType{
			Mode: vnet.NATModeNAT1To1,
		},
		LoggerFactory: loggerFactory,
	})
	assert.NoError(t, err)

	answerNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{answerLocalIP},
	})
	assert.NoError(t, err)
	assert.NoError(t, answerLAN.AddNet(answerNet))
	assert.NoError(t, wan.AddRouter(answerLAN))

	assert.NoError(t, wan.Start())
	defer func() {
		assert.NoError(t, wan.Stop())
	}()

	stunListener, err := stunNet.ListenPacket("udp4", net.JoinHostPort(stunIP, fmt.Sprintf("%d", stunPort)))
	assert.NoError(t, err)

	authKey := turn.GenerateAuthKey("user", "pion.ly", "pass")
	turnServer, err := turn.NewServer(turn.ServerConfig{
		Realm: "pion.ly",
		AuthHandler: func(u, r string, _ net.Addr) ([]byte, bool) {
			return authKey, true
		},
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: stunListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(stunIP),
					Address:      "0.0.0.0",
					Net:          stunNet,
				},
			},
		},
		LoggerFactory: loggerFactory,
	})
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, turnServer.Close())
	}()

	buildSettingEngine := func(n *vnet.Net) SettingEngine {
		se := SettingEngine{}
		se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
		se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
		se.SetSrflxAcceptanceMinWait(wait)
		se.SetICETimeouts(2*time.Second, 4*time.Second, 500*time.Millisecond)
		se.SetNet(n)

		return se
	}

	iceServer := ICEServer{
		URLs: []string{fmt.Sprintf("stun:%s:%d", stunIP, stunPort)},
	}

	offerPC, err := NewAPI(WithSettingEngine(buildSettingEngine(offerNet))).NewPeerConnection(Configuration{
		ICEServers: []ICEServer{iceServer},
	})
	assert.NoError(t, err)

	answerPC, err := NewAPI(WithSettingEngine(buildSettingEngine(answerNet))).NewPeerConnection(Configuration{
		ICEServers: []ICEServer{iceServer},
	})
	assert.NoError(t, err)
	defer closePairNow(t, offerPC, answerPC)

	connected := untilConnectionState(PeerConnectionStateConnected, offerPC, answerPC)

	start := time.Now()
	assert.NoError(t, signalPair(offerPC, answerPC))
	connected.Wait()

	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, wait)
	assert.Less(t, elapsed, 2*wait)
}

func TestICEGatherer_PrflxAcceptanceMinWait(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 40)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		wait                = 300 * time.Millisecond
		defaultPrflxMinWait = time.Second
	)

	pcOffer, pcAnswer, wan := createVNetPair(t, nil)
	defer func() {
		assert.NoError(t, wan.Stop())
		closePairNow(t, pcOffer, pcAnswer)
	}()

	pcOffer.api.settingEngine.timeout.ICEPrflxAcceptanceMinWait = func() *time.Duration {
		d := wait

		return &d
	}()

	var answerCandidate *ICECandidate
	candidateReady := make(chan struct{})
	pcAnswer.OnICECandidate(func(c *ICECandidate) {
		if c == nil || answerCandidate != nil {
			return
		}

		cCopy := *c
		answerCandidate = &cCopy
		close(candidateReady)
	})

	_, err := pcOffer.CreateDataChannel("data", nil)
	assert.NoError(t, err)

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)
	offerGatheringDone := GatheringCompletePromise(pcOffer)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))
	<-offerGatheringDone

	assert.NoError(t, pcAnswer.SetRemoteDescription(*pcOffer.LocalDescription()))

	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)
	answerGatheringDone := GatheringCompletePromise(pcAnswer)
	assert.NoError(t, pcAnswer.SetLocalDescription(answer))
	<-answerGatheringDone

	if answerCandidate == nil {
		<-candidateReady
	}

	filteredAnswer := *pcAnswer.LocalDescription()
	filteredAnswer.SDP = func(sdp string) string {
		lines := strings.Split(sdp, "\n")
		filtered := lines[:0]
		for _, l := range lines {
			if strings.HasPrefix(l, "a=candidate:") || strings.HasPrefix(l, "a=end-of-candidates") {
				continue
			}
			filtered = append(filtered, l)
		}

		return strings.Join(filtered, "\n")
	}(filteredAnswer.SDP)

	assert.NoError(t, pcOffer.SetRemoteDescription(filteredAnswer))

	prflx := *answerCandidate
	prflx.Typ = ICECandidateTypePrflx
	prflx.RelatedAddress = answerCandidate.Address
	prflx.RelatedPort = answerCandidate.Port

	start := time.Now()
	assert.NoError(t, pcOffer.AddICECandidate(prflx.ToJSON()))

	connected := untilConnectionState(PeerConnectionStateConnected, pcOffer, pcAnswer)
	connected.Wait()

	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, wait)
	assert.Less(t, elapsed, defaultPrflxMinWait)
}

func TestICEGatherer_STUNGatherTimeout(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	timeout := 200 * time.Millisecond

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	assert.NoError(t, err)

	net, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{"10.0.0.2"},
	})
	assert.NoError(t, err)

	assert.NoError(t, router.AddNet(net))
	assert.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	se := SettingEngine{}
	se.SetSTUNGatherTimeout(timeout)
	se.SetNet(net)

	opts := ICEGatherOptions{
		ICEServers: []ICEServer{{URLs: []string{"stun:10.0.0.1:9"}}},
	}

	gatherer, err := NewAPI(WithSettingEngine(se)).NewICEGatherer(opts)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, gatherer.Close())
	}()

	gatheringDone := make(chan struct{})
	gatherer.OnLocalCandidate(func(c *ICECandidate) {
		if c == nil {
			close(gatheringDone)
		}
	})

	start := time.Now()
	assert.NoError(t, gatherer.Gather())

	select {
	case <-gatheringDone:
	case <-time.After(3 * time.Second):
		assert.Fail(t, "gathering did not complete")
	}

	assert.LessOrEqual(t, time.Since(start), timeout*10)
}

func TestICEGatherer_RelayAcceptanceMinWait(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 40)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		turnIP              = "10.0.0.1"
		turnPort            = 3478
		username            = "user"
		password            = "pass"
		realm               = "pion.ly"
		defaultRelayMinWait = 2 * time.Second
	)
	wait := 500 * time.Millisecond

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	assert.NoError(t, err)

	turnNet, err := vnet.NewNet(&vnet.NetConfig{StaticIPs: []string{turnIP}})
	assert.NoError(t, err)
	offerNet, err := vnet.NewNet(&vnet.NetConfig{StaticIPs: []string{"10.0.0.2"}})
	assert.NoError(t, err)
	answerNet, err := vnet.NewNet(&vnet.NetConfig{StaticIPs: []string{"10.0.0.3"}})
	assert.NoError(t, err)

	assert.NoError(t, router.AddNet(turnNet))
	assert.NoError(t, router.AddNet(offerNet))
	assert.NoError(t, router.AddNet(answerNet))
	assert.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	turnListener, err := turnNet.ListenPacket("udp4", net.JoinHostPort(turnIP, fmt.Sprintf("%d", turnPort)))
	assert.NoError(t, err)

	authKey := turn.GenerateAuthKey(username, realm, password)
	turnServer, err := turn.NewServer(turn.ServerConfig{
		Realm: realm,
		AuthHandler: func(u, r string, _ net.Addr) ([]byte, bool) {
			if u == username && r == realm {
				return authKey, true
			}

			return nil, false
		},
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: turnListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(turnIP),
					Address:      turnIP,
					Net:          turnNet,
				},
			},
		},
	})
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, turnServer.Close())
	}()

	buildSettingEngine := func(n *vnet.Net) SettingEngine {
		se := SettingEngine{}
		se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
		se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
		se.SetRelayAcceptanceMinWait(wait)
		se.SetICETimeouts(2*time.Second, 4*time.Second, 500*time.Millisecond)
		se.SetNet(n)

		return se
	}

	iceServer := ICEServer{
		URLs:           []string{fmt.Sprintf("turn:%s:%d?transport=udp", turnIP, turnPort)},
		Username:       username,
		Credential:     password,
		CredentialType: ICECredentialTypePassword,
	}

	offerPC, err := NewAPI(WithSettingEngine(buildSettingEngine(offerNet))).NewPeerConnection(Configuration{
		ICEServers:         []ICEServer{iceServer},
		ICETransportPolicy: ICETransportPolicyRelay,
	})
	assert.NoError(t, err)

	answerPC, err := NewAPI(WithSettingEngine(buildSettingEngine(answerNet))).NewPeerConnection(Configuration{
		ICEServers:         []ICEServer{iceServer},
		ICETransportPolicy: ICETransportPolicyRelay,
	})
	assert.NoError(t, err)
	defer closePairNow(t, offerPC, answerPC)

	connected := untilConnectionState(PeerConnectionStateConnected, offerPC, answerPC)

	start := time.Now()
	assert.NoError(t, signalPair(offerPC, answerPC))
	connected.Wait()

	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, wait)
	assert.Less(t, elapsed, defaultRelayMinWait)
}

func TestNewICEGathererSetMediaStreamIdentification(t *testing.T) { //nolint:cyclop
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	opts := ICEGatherOptions{
		ICEServers: []ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}

	gatherer, err := NewAPI().NewICEGatherer(opts)
	assert.NoError(t, err)

	expectedMid := "5"
	expectedMLineIndex := uint16(1)

	gatherer.setMediaStreamIdentification(expectedMid, expectedMLineIndex)

	assert.Equal(t, ICEGathererStateNew, gatherer.State())

	gatherFinished := make(chan struct{})
	gatherer.OnLocalCandidate(func(i *ICECandidate) {
		if i == nil {
			close(gatherFinished)
		} else {
			assert.Equal(t, expectedMid, i.SDPMid)
			assert.Equal(t, expectedMLineIndex, i.SDPMLineIndex)
		}
	})

	assert.NoError(t, gatherer.Gather())
	<-gatherFinished

	params, err := gatherer.GetLocalParameters()
	assert.NoError(t, err)

	assert.NotEmpty(t, params.UsernameFragment, "Empty local username frag")
	assert.NotEmpty(t, params.Password, "Empty local password")

	candidates, err := gatherer.GetLocalCandidates()
	assert.NoError(t, err)
	assert.NotEmpty(t, candidates, "No candidates gathered")

	for _, c := range candidates {
		assert.Equal(t, expectedMid, c.SDPMid)
		assert.Equal(t, expectedMLineIndex, c.SDPMLineIndex)
	}

	assert.NoError(t, gatherer.Close())
}
