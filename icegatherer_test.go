// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/logging"
	"github.com/pion/stun/v3"
	"github.com/pion/transport/v4/test"
	"github.com/pion/transport/v4/vnet"
	"github.com/pion/turn/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestICEGatherer_updateServers(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	gatherer, err := NewAPI().NewICEGatherer(ICEGatherOptions{})
	require.NoError(t, err)

	assert.Equal(t, 0, gatherer.validatedServersCount())

	newServers := []ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}}
	err = gatherer.updateServers(newServers, ICETransportPolicyAll)
	assert.NoError(t, err)
	assert.Equal(t, 1, gatherer.validatedServersCount())

	assert.NoError(t, gatherer.Close())
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
		StaticIPs: []string{localIP},
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

func TestICEAddressRewriteRulesWithNAT1To1Conflict(t *testing.T) {
	lim := test.TimeOut(time.Second * 5)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	t.Run("SetterError", func(t *testing.T) {
		se := SettingEngine{}
		se.SetNAT1To1IPs([]string{"203.0.113.1"}, ICECandidateTypeHost)

		err := se.SetICEAddressRewriteRules(ICEAddressRewriteRule{
			External:        []string{"198.51.100.1"},
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteReplace,
		})
		assert.ErrorIs(t, err, errAddressRewriteWithNAT1To1)
	})

	t.Run("RuntimeError", func(t *testing.T) {
		router, err := vnet.NewRouter(&vnet.RouterConfig{
			CIDR:          "10.0.0.0/24",
			LoggerFactory: logging.NewDefaultLoggerFactory(),
		})
		require.NoError(t, err)

		nw, err := vnet.NewNet(&vnet.NetConfig{
			StaticIPs: []string{"10.0.0.1"},
		})
		require.NoError(t, err)
		require.NoError(t, router.AddNet(nw))
		require.NoError(t, router.Start())
		defer func() {
			assert.NoError(t, router.Stop())
		}()

		se := SettingEngine{}
		se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
		se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
		se.SetNet(nw)
		require.NoError(t, se.SetICEAddressRewriteRules(ICEAddressRewriteRule{
			External:        []string{"198.51.100.2"},
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteReplace,
		}))
		se.SetNAT1To1IPs([]string{"203.0.113.2"}, ICECandidateTypeHost)

		gatherer, err := NewAPI(WithSettingEngine(se)).NewICEGatherer(ICEGatherOptions{})
		require.NoError(t, err)

		err = gatherer.Gather()
		assert.ErrorIs(t, err, errAddressRewriteWithNAT1To1)
		assert.NoError(t, gatherer.Close())
	})
}

func gatherCandidatesWithSettingEngine(t *testing.T, se SettingEngine, opts ICEGatherOptions) []ICECandidate {
	t.Helper()

	gatherer, err := NewAPI(WithSettingEngine(se)).NewICEGatherer(opts)
	require.NoError(t, err)

	done := make(chan struct{})
	var candidates []ICECandidate
	gatherer.OnLocalCandidate(func(c *ICECandidate) {
		if c == nil {
			close(done)

			return
		}
		candidates = append(candidates, *c)
	})

	require.NoError(t, gatherer.Gather())
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "gather did not complete")
	}

	assert.NoError(t, gatherer.Close())

	return candidates
}

func TestICEGatherer_NoHostPolicyVNet(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		stunIP     = "1.2.3.4"
		stunPort   = 3478
		externalIP = "1.2.3.10"
		localIP    = "10.0.0.1"
		realm      = "pion.ly"
		timeout    = 3 * time.Second
	)

	loggerFactory := logging.NewDefaultLoggerFactory()

	wan, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "1.2.3.0/24",
		LoggerFactory: loggerFactory,
	})
	assert.NoError(t, err)

	stunNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{stunIP},
	})
	assert.NoError(t, err)
	assert.NoError(t, wan.AddNet(stunNet))

	clientLAN, err := vnet.NewRouter(&vnet.RouterConfig{
		StaticIPs: []string{fmt.Sprintf("%s/%s", externalIP, localIP)},
		CIDR:      "10.0.0.0/24",
		NATType: &vnet.NATType{
			Mode: vnet.NATModeNAT1To1,
		},
		LoggerFactory: loggerFactory,
	})
	assert.NoError(t, err)

	clientNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{localIP},
	})
	assert.NoError(t, err)
	assert.NoError(t, clientLAN.AddNet(clientNet))
	assert.NoError(t, wan.AddRouter(clientLAN))
	assert.NoError(t, wan.Start())
	defer func() {
		assert.NoError(t, wan.Stop())
	}()

	stunListener, err := stunNet.ListenPacket("udp4", net.JoinHostPort(stunIP, fmt.Sprintf("%d", stunPort)))
	assert.NoError(t, err)

	turnServer, err := turn.NewServer(turn.ServerConfig{
		Realm:         realm,
		LoggerFactory: loggerFactory,
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
	})
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, turnServer.Close())
	}()

	iceServer := ICEServer{
		URLs: []string{fmt.Sprintf("stun:%s:%d", stunIP, stunPort)},
	}

	collect := func(t *testing.T, policy ICETransportPolicy) []ICECandidate {
		t.Helper()

		se := SettingEngine{}
		se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
		se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
		se.SetNet(clientNet)

		gatherer, err := NewAPI(WithSettingEngine(se)).NewICEGatherer(ICEGatherOptions{
			ICEServers:      []ICEServer{iceServer},
			ICEGatherPolicy: policy,
		})
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
		case <-time.After(timeout):
			assert.Fail(t, "gathering did not complete")
		}

		return candidates
	}

	t.Run("All", func(t *testing.T) {
		candidates := collect(t, ICETransportPolicyAll)
		assert.NotEmpty(t, candidates)

		var haveHost, haveSrflx bool
		for _, c := range candidates {
			switch c.Typ {
			case ICECandidateTypeHost:
				haveHost = true
			case ICECandidateTypeSrflx:
				haveSrflx = true
				assert.Equal(t, externalIP, c.Address)
			default:
			}
		}

		assert.True(t, haveHost, "expected host candidate")
		assert.True(t, haveSrflx, "expected srflx candidate")
	})

	t.Run("NoHost", func(t *testing.T) {
		candidates := collect(t, ICETransportPolicyNoHost)
		if assert.NotEmpty(t, candidates) {
			for _, c := range candidates {
				assert.Equal(t, ICECandidateTypeSrflx, c.Typ)
				assert.Equal(t, externalIP, c.Address)
			}
			for _, c := range candidates {
				assert.NotEqual(t, ICECandidateTypeHost, c.Typ)
			}
		}
	})
}

func TestICEGatherer_AddressRewriteRulesVNet(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		externalIP = "203.0.113.10"
		localIP    = "10.0.0.1"
	)

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	require.NoError(t, err)

	nw, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{localIP},
	})
	require.NoError(t, err)
	require.NoError(t, router.AddNet(nw))
	require.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	run := func(rule ICEAddressRewriteRule) []ICECandidate {
		se := SettingEngine{}
		se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
		se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
		se.SetNet(nw)
		require.NoError(t, se.SetICEAddressRewriteRules(rule))

		return gatherCandidatesWithSettingEngine(t, se, ICEGatherOptions{})
	}

	t.Run("HostReplace", func(t *testing.T) {
		candidates := run(ICEAddressRewriteRule{
			External:        []string{externalIP},
			Local:           localIP,
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteReplace,
		})
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
		candidates := run(ICEAddressRewriteRule{
			External:        []string{externalIP},
			AsCandidateType: ICECandidateTypeSrflx,
		})
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

func TestICEGatherer_AddressRewriteRuleFilters(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	t.Run("CIDR", func(t *testing.T) {
		const (
			firstIP    = "10.0.0.2"
			secondIP   = "10.0.1.2"
			externalIP = "203.0.113.20"
		)

		router, err := vnet.NewRouter(&vnet.RouterConfig{
			CIDR:          "10.0.0.0/16",
			LoggerFactory: logging.NewDefaultLoggerFactory(),
		})
		require.NoError(t, err)

		nw, err := vnet.NewNet(&vnet.NetConfig{
			StaticIPs: []string{firstIP, secondIP},
		})
		require.NoError(t, err)
		require.NoError(t, router.AddNet(nw))
		require.NoError(t, router.Start())
		defer func() {
			assert.NoError(t, router.Stop())
		}()

		se := SettingEngine{}
		se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
		se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
		se.SetNet(nw)
		require.NoError(t, se.SetICEAddressRewriteRules(ICEAddressRewriteRule{
			External:        []string{externalIP},
			CIDR:            "10.0.0.0/24",
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteReplace,
		}))

		candidates := gatherCandidatesWithSettingEngine(t, se, ICEGatherOptions{})

		var hostAddrs []string
		for _, c := range candidates {
			if c.Typ == ICECandidateTypeHost {
				hostAddrs = append(hostAddrs, c.Address)
			}
		}

		assert.Contains(t, hostAddrs, externalIP)
		assert.Contains(t, hostAddrs, secondIP)
		assert.NotContains(t, hostAddrs, firstIP)
	})

	t.Run("NetworkTypes", func(t *testing.T) {
		const (
			localIP    = "10.0.0.50"
			externalIP = "203.0.113.50"
		)

		router, err := vnet.NewRouter(&vnet.RouterConfig{
			CIDR:          "10.0.0.0/24",
			LoggerFactory: logging.NewDefaultLoggerFactory(),
		})
		require.NoError(t, err)

		nw, err := vnet.NewNet(&vnet.NetConfig{
			StaticIPs: []string{localIP},
		})
		require.NoError(t, err)
		require.NoError(t, router.AddNet(nw))
		require.NoError(t, router.Start())
		defer func() {
			assert.NoError(t, router.Stop())
		}()

		se := SettingEngine{}
		se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
		se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
		se.SetNet(nw)
		require.NoError(t, se.SetICEAddressRewriteRules(ICEAddressRewriteRule{
			External:        []string{externalIP},
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteReplace,
			Networks:        []NetworkType{NetworkTypeUDP6},
		}))

		candidates := gatherCandidatesWithSettingEngine(t, se, ICEGatherOptions{})

		var hostAddrs []string
		for _, c := range candidates {
			if c.Typ == ICECandidateTypeHost {
				hostAddrs = append(hostAddrs, c.Address)
			}
		}

		assert.Contains(t, hostAddrs, localIP)
		assert.NotContains(t, hostAddrs, externalIP)
	})
}

func TestICEGatherer_AddressRewriteHostAppendAndReplace(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		firstLocal     = "10.0.0.2"
		secondLocal    = "10.0.0.3"
		firstExternal  = "203.0.113.30"
		secondExternal = "203.0.113.31"
	)

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	require.NoError(t, err)

	nw, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{firstLocal, secondLocal},
	})
	require.NoError(t, err)
	require.NoError(t, router.AddNet(nw))
	require.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	se := SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	se.SetNet(nw)
	require.NoError(t, se.SetICEAddressRewriteRules(
		ICEAddressRewriteRule{
			Local:           firstLocal,
			External:        []string{firstExternal},
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteReplace,
		},
		ICEAddressRewriteRule{
			Local:           secondLocal,
			External:        []string{secondExternal},
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteAppend,
		},
	))

	candidates := gatherCandidatesWithSettingEngine(t, se, ICEGatherOptions{})

	var hostAddrs []string
	for _, c := range candidates {
		if c.Typ == ICECandidateTypeHost {
			hostAddrs = append(hostAddrs, c.Address)
		}
	}

	assert.Contains(t, hostAddrs, firstExternal)
	assert.NotContains(t, hostAddrs, firstLocal)
	assert.Contains(t, hostAddrs, secondLocal)
	assert.Contains(t, hostAddrs, secondExternal)
}

func TestICEGatherer_AddressRewriteSrflxReplace(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		localIP    = "10.0.0.60"
		externalIP = "203.0.113.60"
	)

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	require.NoError(t, err)

	nw, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{localIP},
	})
	require.NoError(t, err)
	require.NoError(t, router.AddNet(nw))
	require.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	se := SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	se.SetNet(nw)
	require.NoError(t, se.SetICEAddressRewriteRules(ICEAddressRewriteRule{
		External:        []string{externalIP},
		AsCandidateType: ICECandidateTypeSrflx,
		Mode:            ICEAddressRewriteReplace,
	}))

	candidates := gatherCandidatesWithSettingEngine(t, se, ICEGatherOptions{})

	var hostAddrs []string
	var srflxAddrs []string
	for _, c := range candidates {
		switch c.Typ {
		case ICECandidateTypeHost:
			hostAddrs = append(hostAddrs, c.Address)
		case ICECandidateTypeSrflx:
			srflxAddrs = append(srflxAddrs, c.Address)
		default:
			t.Logf("unexpected candidate type: %s", c.Typ)
		}
	}

	assert.Contains(t, hostAddrs, localIP)
	assert.Contains(t, srflxAddrs, externalIP)
	assert.NotContains(t, srflxAddrs, localIP)
}

func TestICEGatherer_AddressRewriteSrflxAppendWithCatchAll(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		localIP   = "10.0.0.80"
		appendIP  = "203.0.113.81"
		replaceIP = "203.0.113.80"
	)

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	require.NoError(t, err)

	nw, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{localIP},
	})
	require.NoError(t, err)
	require.NoError(t, router.AddNet(nw))
	require.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	se := SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	se.SetNet(nw)
	require.NoError(t, se.SetICEAddressRewriteRules(
		ICEAddressRewriteRule{
			External:        []string{appendIP},
			AsCandidateType: ICECandidateTypeSrflx,
			Mode:            ICEAddressRewriteAppend,
		},
		ICEAddressRewriteRule{
			External:        []string{replaceIP},
			AsCandidateType: ICECandidateTypeSrflx,
			Mode:            ICEAddressRewriteReplace,
		},
	))

	candidates := gatherCandidatesWithSettingEngine(t, se, ICEGatherOptions{})

	var srflxAddrs []string
	for _, c := range candidates {
		if c.Typ == ICECandidateTypeSrflx {
			srflxAddrs = append(srflxAddrs, c.Address)
		}
	}

	assert.Contains(t, srflxAddrs, appendIP)
	assert.NotContains(t, srflxAddrs, replaceIP)
	assert.NotContains(t, srflxAddrs, localIP)
}

func TestICEGatherer_AddressRewriteMultipleRulesOrdering(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		localIP      = "10.0.0.70"
		otherLocalIP = "10.0.0.71"
		externalIP   = "203.0.113.70"
	)

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	require.NoError(t, err)

	nw, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{localIP, otherLocalIP},
	})
	require.NoError(t, err)
	require.NoError(t, router.AddNet(nw))
	require.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	se := SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	se.SetNet(nw)
	require.NoError(t, se.SetICEAddressRewriteRules(
		ICEAddressRewriteRule{
			CIDR:            "10.0.0.0/24",
			External:        []string{externalIP},
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteReplace,
		},
		ICEAddressRewriteRule{
			Local:           otherLocalIP,
			External:        []string{otherLocalIP},
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteAppend,
		},
	))

	candidates := gatherCandidatesWithSettingEngine(t, se, ICEGatherOptions{})

	var hostAddrs []string
	for _, c := range candidates {
		if c.Typ == ICECandidateTypeHost {
			hostAddrs = append(hostAddrs, c.Address)
		}
	}

	assert.Contains(t, hostAddrs, externalIP)
	assert.NotContains(t, hostAddrs, localIP)
	assert.Contains(t, hostAddrs, otherLocalIP)
}

func TestICEGatherer_AddressRewriteIfaceScope(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		localIP    = "10.0.0.90"
		externalIP = "203.0.113.90"
	)

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	require.NoError(t, err)

	nw, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{localIP},
	})
	require.NoError(t, err)
	require.NoError(t, router.AddNet(nw))
	require.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	se := SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	se.SetNet(nw)
	require.NoError(t, se.SetICEAddressRewriteRules(
		ICEAddressRewriteRule{
			Iface:           "bad0",
			External:        []string{"198.51.100.90"},
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteReplace,
		},
		ICEAddressRewriteRule{
			Iface:           "eth0",
			External:        []string{externalIP},
			AsCandidateType: ICECandidateTypeHost,
			Mode:            ICEAddressRewriteReplace,
		},
	))

	candidates := gatherCandidatesWithSettingEngine(t, se, ICEGatherOptions{})

	var hostAddrs []string
	for _, c := range candidates {
		if c.Typ == ICECandidateTypeHost {
			hostAddrs = append(hostAddrs, c.Address)
		}
	}

	assert.Contains(t, hostAddrs, externalIP)
	assert.NotContains(t, hostAddrs, localIP)
	assert.NotContains(t, hostAddrs, "198.51.100.90")
}

func TestICEConnection_AddressRewriteAppend(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 15)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		offerIP       = "1.2.3.4"
		answerIP      = "1.2.3.5"
		offerExternal = "203.0.113.200"
	)

	wan, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "1.2.3.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	require.NoError(t, err)

	offerNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{offerIP},
	})
	require.NoError(t, err)
	answerNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{answerIP},
	})
	require.NoError(t, err)

	require.NoError(t, wan.AddNet(offerNet))
	require.NoError(t, wan.AddNet(answerNet))
	require.NoError(t, wan.Start())
	defer func() {
		assert.NoError(t, wan.Stop())
	}()

	offerSE := SettingEngine{}
	offerSE.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	offerSE.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	offerSE.SetNet(offerNet)
	require.NoError(t, offerSE.SetICEAddressRewriteRules(ICEAddressRewriteRule{
		External:        []string{offerExternal},
		AsCandidateType: ICECandidateTypeHost,
		Mode:            ICEAddressRewriteAppend,
	}))

	answerSE := SettingEngine{}
	answerSE.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	answerSE.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	answerSE.SetNet(answerNet)

	offerPC, err := NewAPI(WithSettingEngine(offerSE)).NewPeerConnection(Configuration{})
	require.NoError(t, err)
	answerPC, err := NewAPI(WithSettingEngine(answerSE)).NewPeerConnection(Configuration{})
	require.NoError(t, err)
	defer closePairNow(t, offerPC, answerPC)

	var offerCandidates []ICECandidate
	offerPC.OnICECandidate(func(c *ICECandidate) {
		if c != nil {
			offerCandidates = append(offerCandidates, *c)
		}
	})

	assert.NoError(t, signalPair(offerPC, answerPC))

	connected := untilConnectionState(PeerConnectionStateConnected, offerPC, answerPC)
	connected.Wait()

	var hostAddrs []string
	for _, c := range offerCandidates {
		if c.Typ == ICECandidateTypeHost {
			hostAddrs = append(hostAddrs, c.Address)
		}
	}

	assert.Contains(t, hostAddrs, offerIP)
	assert.Contains(t, hostAddrs, offerExternal)
}

func TestICEAddressRewriteDropRule(t *testing.T) {
	se := SettingEngine{}

	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})

	err := se.SetICEAddressRewriteRules(ICEAddressRewriteRule{
		External:        nil,
		AsCandidateType: ICECandidateTypeHost,
		Mode:            ICEAddressRewriteReplace,
	})
	assert.NoError(t, err, "rule is allowed to be configured, validation happens in ice")

	gatherer, gErr := NewAPI(WithSettingEngine(se)).NewICEGatherer(ICEGatherOptions{})
	require.NoError(t, gErr)
	defer func() {
		assert.NoError(t, gatherer.Close())
	}()

	assert.ErrorIs(t, gatherer.Gather(), ice.ErrInvalidAddressRewriteMapping)
}

func TestICEGatherer_AddressRewriteRelayVNet(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 15)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		turnIP         = "10.0.0.2"
		clientIP       = "10.0.0.3"
		relayExternal  = "203.0.113.77"
		turnListenPort = "3478"
	)

	loggerFactory := logging.NewDefaultLoggerFactory()

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: loggerFactory,
	})
	require.NoError(t, err)

	turnNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{turnIP},
	})
	require.NoError(t, err)
	clientNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{clientIP},
	})
	require.NoError(t, err)

	require.NoError(t, router.AddNet(turnNet))
	require.NoError(t, router.AddNet(clientNet))
	require.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	turnListener, err := turnNet.ListenPacket("udp4", net.JoinHostPort(turnIP, turnListenPort))
	require.NoError(t, err)

	authKey := turn.GenerateAuthKey("user", "pion.ly", "pass")
	turnServer, err := turn.NewServer(turn.ServerConfig{
		Realm: "pion.ly",
		AuthHandler: func(u, r string, _ net.Addr) ([]byte, bool) {
			if u == "user" && r == "pion.ly" {
				return authKey, true
			}

			return nil, false
		},
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: turnListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(turnIP),
					Address:      "0.0.0.0",
					Net:          turnNet,
				},
			},
		},
		LoggerFactory: loggerFactory,
	})
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, turnServer.Close())
	}()

	se := SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	se.SetNet(clientNet)
	require.NoError(t, se.SetICEAddressRewriteRules(ICEAddressRewriteRule{
		External:        []string{relayExternal},
		AsCandidateType: ICECandidateTypeRelay,
		Mode:            ICEAddressRewriteReplace,
	}))

	candidates := gatherCandidatesWithSettingEngine(t, se, ICEGatherOptions{
		ICEServers: []ICEServer{
			{
				URLs:       []string{fmt.Sprintf("turn:%s:%s?transport=udp", turnIP, turnListenPort)},
				Username:   "user",
				Credential: "pass",
			},
		},
		ICEGatherPolicy: ICETransportPolicyRelay,
	})

	var relayAddrs []string
	for _, c := range candidates {
		if c.Typ == ICECandidateTypeRelay {
			relayAddrs = append(relayAddrs, c.Address)
		}
	}

	assert.NotEmpty(t, relayAddrs, "expected relay candidates")
	assert.Subset(t, relayAddrs, []string{relayExternal})
	assert.NotContains(t, relayAddrs, turnIP)
}

func TestICEGatherer_AddressRewriteRelayAppendVNet(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 15)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	const (
		turnIP         = "10.0.0.4"
		clientIP       = "10.0.0.5"
		relayExternal  = "203.0.113.78"
		turnListenPort = "3478"
	)

	loggerFactory := logging.NewDefaultLoggerFactory()

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: loggerFactory,
	})
	require.NoError(t, err)

	turnNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{turnIP},
	})
	require.NoError(t, err)
	clientNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{clientIP},
	})
	require.NoError(t, err)

	require.NoError(t, router.AddNet(turnNet))
	require.NoError(t, router.AddNet(clientNet))
	require.NoError(t, router.Start())
	defer func() {
		assert.NoError(t, router.Stop())
	}()

	turnListener, err := turnNet.ListenPacket("udp4", net.JoinHostPort(turnIP, turnListenPort))
	require.NoError(t, err)

	authKey := turn.GenerateAuthKey("user", "pion.ly", "pass")
	turnServer, err := turn.NewServer(turn.ServerConfig{
		Realm: "pion.ly",
		AuthHandler: func(u, r string, _ net.Addr) ([]byte, bool) {
			if u == "user" && r == "pion.ly" {
				return authKey, true
			}

			return nil, false
		},
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: turnListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(turnIP),
					Address:      "0.0.0.0",
					Net:          turnNet,
				},
			},
		},
		LoggerFactory: loggerFactory,
	})
	require.NoError(t, err)

	se := SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	se.SetNet(clientNet)
	require.NoError(t, se.SetICEAddressRewriteRules(ICEAddressRewriteRule{
		External:        []string{relayExternal},
		AsCandidateType: ICECandidateTypeRelay,
		Mode:            ICEAddressRewriteAppend,
	}))

	candidates := gatherCandidatesWithSettingEngine(t, se, ICEGatherOptions{
		ICEServers: []ICEServer{
			{
				URLs:       []string{fmt.Sprintf("turn:%s:%s?transport=udp", turnIP, turnListenPort)},
				Username:   "user",
				Credential: "pass",
			},
		},
		ICEGatherPolicy: ICETransportPolicyRelay,
	})

	var relayAddrs []string
	for _, c := range candidates {
		if c.Typ == ICECandidateTypeRelay {
			relayAddrs = append(relayAddrs, c.Address)
		}
	}

	assert.Contains(t, relayAddrs, turnIP)
	assert.Contains(t, relayAddrs, relayExternal)

	if err := turnServer.Close(); err != nil {
		t.Logf("turn server close: %v", err)
	}
	if err := turnListener.Close(); err != nil {
		t.Logf("turn listener close: %v", err)
	}
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
		StaticIPs: []string{stunIP},
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

func TestICEGatherer_RenominationOptions(t *testing.T) {
	se := SettingEngine{}
	assert.NoError(t, se.SetICERenomination())
	assert.True(t, se.renomination.enabled)
	assert.True(t, se.renomination.automatic)
	assert.Nil(t, se.renomination.automaticInterval)
	assert.NotNil(t, se.renomination.generator)
}

func TestICEGatherer_RenominationOptionsDisabled(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	offerPC, answerPC, cleanup := buildRenominationVNetPair(t, false, false, nil)
	defer cleanup()

	connectAndWaitForICE(t, offerPC, answerPC)

	agent := getAgent(t, offerPC)

	selectedPair, err := agent.GetSelectedCandidatePair()
	assert.NoError(t, err)
	assert.NotNil(t, selectedPair)

	err = agent.RenominateCandidate(selectedPair.Local, selectedPair.Remote)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ice.ErrRenominationNotEnabled)
}

func TestICEGatherer_RenominationSendsNomination(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 35)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	nominationCh := make(chan uint32, 2)
	handler := func(m *stun.Message, _, _ ice.Candidate, _ *ice.CandidatePair) bool {
		var attr ice.NominationAttribute
		if err := attr.GetFrom(m); err == nil {
			select {
			case nominationCh <- attr.Value:
			default:
			}
		}

		return false
	}

	offerPC, answerPC, offerSender, answerSender, cleanup := buildStagedRenominationPair(t, handler)
	defer cleanup()

	recvCh := make(chan string, 4)
	negotiated := true
	id := uint16(0)
	offerDC, err := offerPC.CreateDataChannel("renomination-dc", &DataChannelInit{
		Negotiated: &negotiated,
		ID:         &id,
	})
	assert.NoError(t, err)
	answerDC, err := answerPC.CreateDataChannel("renomination-dc", &DataChannelInit{
		Negotiated: &negotiated,
		ID:         &id,
	})
	assert.NoError(t, err)
	answerDC.OnMessage(func(msg DataChannelMessage) {
		select {
		case recvCh <- string(msg.Data):
		default:
		}
	})

	connected := make(chan struct{})
	var once sync.Once
	offerPC.OnICEConnectionStateChange(func(state ICEConnectionState) {
		if state == ICEConnectionStateConnected {
			once.Do(func() {
				close(connected)
			})
		}
	})

	startTrickleRenomination(t, offerPC, answerPC, offerSender, answerSender)
	assert.NoError(t, offerSender.errValue())
	assert.NoError(t, answerSender.errValue())

	select {
	case <-connected:
	case <-time.After(15 * time.Second):
		assert.Fail(t, "timed out waiting for ICE to connect")
	}

	pair := selectedCandidatePair(t, offerPC)
	assert.NotNil(t, pair)
	if pair.Remote.Type() != ice.CandidateTypeServerReflexive {
		t.Logf("initial remote candidate type %s (expected srflx), continuing", pair.Remote.Type())
	}
	initialStat, initialStatOK := getAgent(t, offerPC).GetSelectedCandidatePairStats()
	assert.True(t, initialStatOK)
	assert.NoError(t, offerSender.flushHost())
	assert.NoError(t, answerSender.flushHost())

	waitDataChannelOpen(t, offerDC)
	waitDataChannelOpen(t, answerDC)
	sendAndExpect(t, offerDC, recvCh, "before-renom")

	waitForTwoRemoteCandidates(t, offerPC)
	waitForTwoRemoteCandidates(t, answerPC)

	var switchLocal ice.Candidate
	var switchRemote ice.Candidate
	agent := getAgent(t, offerPC)
	assert.Eventuallyf(t, func() bool {
		switchLocal, switchRemote = findSwitchTarget(t, offerPC, initialStat.RemoteCandidateID)

		return switchLocal != nil && switchRemote != nil
	}, 10*time.Second, 50*time.Millisecond, "no alternate succeeded pair found; pairs: %s", candidatePairSummary(t, agent))
	assert.NoError(t, agent.RenominateCandidate(switchLocal, switchRemote))

	sendAndExpect(t, offerDC, recvCh, "after-renom")

	select {
	case v := <-nominationCh:
		assert.Greater(t, v, uint32(0))
	case <-time.After(20 * time.Second):
		assert.Fail(t, "did not observe nomination attribute on binding request")
	}
}

func TestICEGatherer_RenominationSwitchesPair(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 45)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	offerPC, answerPC, offerSender, answerSender, cleanup := buildStagedRenominationPair(t, nil)
	defer cleanup()

	recvCh := make(chan string, 4)
	negotiated := true
	id := uint16(0)
	offerDC, err := offerPC.CreateDataChannel("renomination-dc", &DataChannelInit{
		Negotiated: &negotiated,
		ID:         &id,
	})
	assert.NoError(t, err)
	answerDC, err := answerPC.CreateDataChannel("renomination-dc", &DataChannelInit{
		Negotiated: &negotiated,
		ID:         &id,
	})
	assert.NoError(t, err)
	answerDC.OnMessage(func(msg DataChannelMessage) {
		select {
		case recvCh <- string(msg.Data):
		default:
		}
	})

	connected := make(chan struct{})
	offerPC.OnICEConnectionStateChange(func(state ICEConnectionState) {
		if state == ICEConnectionStateConnected {
			select {
			case <-connected:
			default:
				close(connected)
			}
		}
	})

	var flushHostOnce sync.Once
	flushHosts := func() {
		flushHostOnce.Do(func() {
			assert.NoError(t, offerSender.flushHost())
			assert.NoError(t, answerSender.flushHost())
		})
	}

	startTrickleRenomination(t, offerPC, answerPC, offerSender, answerSender)
	assert.NoError(t, offerSender.errValue())
	assert.NoError(t, answerSender.errValue())

	// Fallback: release host candidates even if the initial selection check stalls.
	go func() {
		time.Sleep(time.Second)
		flushHosts()
	}()

	select {
	case <-connected:
	case <-time.After(15 * time.Second):
		agent := getAgent(t, offerPC)
		assert.Fail(t, "timed out waiting for initial connection; pairs: %s", candidatePairSummary(t, agent))
	}

	var initialRemoteType ice.CandidateType
	if !assert.Eventuallyf(
		t, func() bool {
			if pair := selectedCandidatePair(t, offerPC); pair == nil {
				return false
			} else {
				initialRemoteType = pair.Remote.Type()

				return initialRemoteType == ice.CandidateTypeServerReflexive ||
					initialRemoteType == ice.CandidateTypePeerReflexive
			}
		},
		12*time.Second, 30*time.Millisecond,
		"expected to start on a srflx/prflx remote candidate (got %s)", initialRemoteType,
	) {
		flushHosts()
		assert.Fail(t, "expected to start on a srflx/prflx remote candidate")
	}

	flushHosts()

	waitDataChannelOpen(t, offerDC)
	waitDataChannelOpen(t, answerDC)
	sendAndExpect(t, offerDC, recvCh, "before-switch")

	initialPair := selectedCandidatePair(t, offerPC)
	initialStat, initialStatOK := getAgent(t, offerPC).GetSelectedCandidatePairStats()
	t.Logf("initial selected pair: %s<->%s (%s/%s)",
		initialPair.Local.Address(), initialPair.Remote.Address(), initialPair.Local.Type(), initialPair.Remote.Type())

	waitForTwoRemoteCandidates(t, offerPC)
	waitForTwoRemoteCandidates(t, answerPC)

	assert.True(t, initialStatOK, "missing initial selected pair stats")

	switchLocal, switchRemote := findSwitchTarget(t, offerPC, initialStat.RemoteCandidateID)
	assert.NotNil(t, switchLocal)
	assert.NotNil(t, switchRemote)
	assert.NotNil(t, switchLocal.Type())
	assert.NotNil(t, switchRemote.Type())
	assert.False(t, switchLocal.Equal(switchRemote), "switch local and remote candidates should be different")

	t.Logf(
		"renomination target: %s/%s -> %s/%s",
		switchLocal.Address(), switchLocal.Type(), switchRemote.Address(), switchRemote.Type(),
	)

	agent := getAgent(t, offerPC)
	if !assert.Eventually(t, func() bool {
		pair := selectedCandidatePair(t, offerPC)
		if pair != nil && pair.Local.Equal(switchLocal) && pair.Remote.Equal(switchRemote) {
			return true
		}

		if err := agent.RenominateCandidate(switchLocal, switchRemote); err != nil {
			t.Logf("renomination attempt: %v", err)
		}

		return false
	}, 10*time.Second, 50*time.Millisecond, "selected pair should change after renomination") {
		assert.Fail(t, "selected pair did not switch; pairs: %s", candidatePairSummary(t, agent))
	}

	finalStat, ok := agent.GetSelectedCandidatePairStats()
	assert.True(t, ok)
	assert.NotEqual(
		t, initialStat.RemoteCandidateID, finalStat.RemoteCandidateID, "selected pair should change after renomination",
	)

	finalLocal := findCandidateByID(t, agent, finalStat.LocalCandidateID, true)
	finalRemote := findCandidateByID(t, agent, finalStat.RemoteCandidateID, false)
	assert.NotNil(t, finalLocal)
	assert.NotNil(t, finalRemote)
	assert.Equal(t, ice.CandidateTypeHost, finalLocal.Type())
	assert.NotEqual(t, ice.CandidateTypeServerReflexive, finalRemote.Type())

	finalPair := selectedCandidatePair(t, offerPC)
	assert.NotNil(t, finalPair)
	sendAndExpect(t, offerDC, recvCh, "after-switch")
	assert.False(t, initialPair.Remote.Equal(finalPair.Remote), "expected remote candidate to change after renomination")
}

func TestICEGatherer_GracefulCloseDuringAgentActivity(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	gatherer, err := NewAPI().NewICEGatherer(ICEGatherOptions{})
	assert.NoError(t, err)

	onStateChangeCalled := make(chan struct{})

	gatherer.OnStateChange(func(state ICEGathererState) {
		if state == ICEGathererStateComplete {
			close(onStateChangeCalled)

			// Yield the agent goroutine long enough for GracefulClose
			// to acquire g.lock before we return and hit g.lock too.
			time.Sleep(50 * time.Millisecond)
		}
	})

	err = gatherer.Gather()
	assert.NoError(t, err)

	<-onStateChangeCalled

	err = gatherer.GracefulClose()
	assert.NoError(t, err)
}

func buildRenominationVNetPair(
	t *testing.T,
	enableRenomination bool,
	automatic bool,
	bindingHandler func(*stun.Message, ice.Candidate, ice.Candidate, *ice.CandidatePair) bool,
) (*PeerConnection, *PeerConnection, func()) {
	t.Helper()

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "1.2.3.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	assert.NoError(t, err)

	netStack, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{"1.2.3.4"},
	})
	assert.NoError(t, err)
	assert.NoError(t, router.AddNet(netStack))

	answerNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{"1.2.3.5"},
	})
	assert.NoError(t, err)
	assert.NoError(t, router.AddNet(answerNet))

	assert.NoError(t, router.Start())

	offerSE := SettingEngine{}
	offerSE.SetNet(netStack)
	offerSE.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	offerSE.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	if enableRenomination {
		assert.NoError(t, offerSE.SetICERenomination())
		if automatic {
			assert.NoError(t, offerSE.SetICERenomination())
		}
	}

	answerSE := SettingEngine{}
	answerSE.SetNet(answerNet)
	answerSE.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	answerSE.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	if enableRenomination {
		assert.NoError(t, answerSE.SetICERenomination())
		if automatic {
			assert.NoError(t, answerSE.SetICERenomination())
		}
	}
	if bindingHandler != nil {
		answerSE.SetICEBindingRequestHandler(bindingHandler)
	}

	offerPC, err := NewAPI(WithSettingEngine(offerSE)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	answerPC, err := NewAPI(WithSettingEngine(answerSE)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	cleanup := func() {
		closePairNow(t, offerPC, answerPC)
		assert.NoError(t, router.Stop())
	}

	return offerPC, answerPC, cleanup
}

func connectAndWaitForICE(t *testing.T, offerPC, answerPC *PeerConnection) {
	t.Helper()

	connected := make(chan struct{})
	var once sync.Once
	offerPC.OnICEConnectionStateChange(func(state ICEConnectionState) {
		if state == ICEConnectionStateConnected {
			once.Do(func() {
				close(connected)
			})
		}
	})

	assert.NoError(t, signalPair(offerPC, answerPC))

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "timed out waiting for ICE to connect")
	}
}

func selectedCandidatePair(t *testing.T, pc *PeerConnection) *ice.CandidatePair {
	t.Helper()

	agent := getAgent(t, pc)

	pair, err := agent.GetSelectedCandidatePair()
	assert.NoError(t, err)

	return pair
}

func waitForTwoRemoteCandidates(t *testing.T, pc *PeerConnection) {
	t.Helper()

	assert.Eventually(t, func() bool {
		agent := getAgent(t, pc)

		remotes, err := agent.GetRemoteCandidates()
		assert.NoError(t, err)

		return len(remotes) >= 2
	}, 5*time.Second, 20*time.Millisecond)
}

func findCandidateByID(t *testing.T, agent *ice.Agent, id string, local bool) ice.Candidate {
	t.Helper()

	var cands []ice.Candidate
	var err error
	if local {
		cands, err = agent.GetLocalCandidates()
	} else {
		cands, err = agent.GetRemoteCandidates()
	}
	assert.NoError(t, err)

	for _, cand := range cands {
		if cand.ID() == id {
			return cand
		}
	}

	return nil
}

//nolint:cyclop
func findSwitchTarget(
	t *testing.T, pc *PeerConnection, excludeRemoteID string,
) (ice.Candidate, ice.Candidate) {
	t.Helper()

	agent := getAgent(t, pc)
	var targetLocal ice.Candidate
	var targetRemote ice.Candidate

	for _, stat := range agent.GetCandidatePairsStats() {
		if stat.State != ice.CandidatePairStateSucceeded ||
			stat.LocalCandidateID == "" || stat.RemoteCandidateID == "" ||
			stat.RemoteCandidateID == excludeRemoteID {
			continue
		}

		local := findCandidateByID(t, agent, stat.LocalCandidateID, true)
		remote := findCandidateByID(t, agent, stat.RemoteCandidateID, false)
		if local == nil || remote == nil {
			continue
		}

		if local.Type() != ice.CandidateTypeHost {
			continue
		}

		if remote.Type() == ice.CandidateTypeHost {
			return local, remote
		}

		if remote.Type() == ice.CandidateTypePeerReflexive {
			targetLocal = local
			targetRemote = remote
		}
	}

	return targetLocal, targetRemote
}

func getAgent(t *testing.T, pc *PeerConnection) *ice.Agent {
	t.Helper()

	pc.iceTransport.lock.RLock()
	agent := pc.iceTransport.gatherer.getAgent()
	pc.iceTransport.lock.RUnlock()
	assert.NotNil(t, agent)

	return agent
}

func candidatePairSummary(t *testing.T, agent *ice.Agent) string {
	t.Helper()

	locals, err := agent.GetLocalCandidates()
	assert.NoError(t, err)
	remotes, err := agent.GetRemoteCandidates()
	assert.NoError(t, err)

	localMap := map[string]string{}
	for _, cand := range locals {
		localMap[cand.ID()] = fmt.Sprintf("%s/%s", cand.Address(), cand.Type())
	}

	remoteMap := map[string]string{}
	for _, cand := range remotes {
		remoteMap[cand.ID()] = fmt.Sprintf("%s/%s", cand.Address(), cand.Type())
	}

	stats := agent.GetCandidatePairsStats()
	summary := make([]string, 0, len(stats))
	for _, stat := range stats {
		summary = append(summary, fmt.Sprintf(
			"%s<->%s state=%s nominated=%v rtt=%.2fms",
			localMap[stat.LocalCandidateID],
			remoteMap[stat.RemoteCandidateID],
			stat.State,
			stat.Nominated,
			stat.CurrentRoundTripTime*1000,
		))
	}

	return strings.Join(summary, "; ")
}

func waitDataChannelOpen(t *testing.T, dc *DataChannel) {
	t.Helper()

	if dc.ReadyState() == DataChannelStateOpen {
		return
	}

	done := make(chan struct{})
	dc.OnOpen(func() {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "data channel did not open")
	}
}

func sendAndExpect(t *testing.T, sender *DataChannel, recvCh chan string, msg string) {
	t.Helper()

	err := sender.SendText(msg)
	assert.NoError(t, err)

	select {
	case got := <-recvCh:
		assert.Equal(t, msg, got)
	case <-time.After(5 * time.Second):
		assert.Fail(t, "did not receive data channel message")
	}
}

type stagedCandidateSender struct {
	remote *PeerConnection
	mu     sync.Mutex
	srflx  []ICECandidateInit
	host   []ICECandidateInit
	err    error
}

func (s *stagedCandidateSender) addCandidate(cand ICECandidateInit, srflx bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return
	}

	if srflx && s.remote.RemoteDescription() != nil {
		if err := s.remote.AddICECandidate(cand); err != nil {
			s.err = err
		}

		return
	}

	if srflx {
		s.srflx = append(s.srflx, cand)
	} else {
		s.host = append(s.host, cand)
	}
}

func (s *stagedCandidateSender) flushSrflx() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}

	for _, cand := range s.srflx {
		if err := s.remote.AddICECandidate(cand); err != nil {
			s.err = err

			return err
		}
	}

	s.srflx = nil

	return s.err
}

func (s *stagedCandidateSender) flushHost() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}

	for _, cand := range s.host {
		if err := s.remote.AddICECandidate(cand); err != nil {
			s.err = err

			return err
		}
	}

	s.host = nil

	return s.err
}

func (s *stagedCandidateSender) errValue() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.err
}

func makeSrflxCandidateInit(c ICECandidate) ICECandidateInit {
	init := c.ToJSON()
	replacement := fmt.Sprintf("typ srflx raddr %s rport %d", c.Address, c.Port)
	init.Candidate = strings.Replace(init.Candidate, "typ host", replacement, 1)

	return init
}

func buildStagedRenominationPair(
	t *testing.T,
	bindingHandler func(*stun.Message, ice.Candidate, ice.Candidate, *ice.CandidatePair) bool,
) (*PeerConnection, *PeerConnection, *stagedCandidateSender, *stagedCandidateSender, func()) {
	t.Helper()

	const (
		primaryOfferIP    = "10.0.0.2"
		secondaryOfferIP  = "10.0.0.4"
		primaryAnswerIP   = "10.0.0.3"
		secondaryAnswerIP = "10.0.0.5"
	)

	router, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "10.0.0.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	assert.NoError(t, err)

	offerNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{primaryOfferIP, secondaryOfferIP},
	})
	assert.NoError(t, err)
	assert.NoError(t, router.AddNet(offerNet))

	answerNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{primaryAnswerIP, secondaryAnswerIP},
	})
	assert.NoError(t, err)
	assert.NoError(t, router.AddNet(answerNet))

	assert.NoError(t, router.Start())

	offerSE := SettingEngine{}
	offerSE.SetNet(offerNet)
	offerSE.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	offerSE.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	offerSE.SetICETimeouts(5*time.Second, 15*time.Second, 200*time.Millisecond)
	// prefer srflx/prflx nomination first so the test reliably observes the switch to host via renomination.
	offerSE.SetSrflxAcceptanceMinWait(0)
	offerSE.SetHostAcceptanceMinWait(3 * time.Second)
	assert.NoError(t, offerSE.SetICERenomination(WithRenominationInterval(200*time.Millisecond)))

	answerSE := SettingEngine{}
	answerSE.SetNet(answerNet)
	answerSE.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	answerSE.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
	answerSE.SetICETimeouts(5*time.Second, 15*time.Second, 200*time.Millisecond)
	answerSE.SetSrflxAcceptanceMinWait(0)
	answerSE.SetHostAcceptanceMinWait(3 * time.Second)
	assert.NoError(t, answerSE.SetICERenomination(WithRenominationInterval(200*time.Millisecond)))
	if bindingHandler != nil {
		answerSE.SetICEBindingRequestHandler(bindingHandler)
	}

	offerPC, err := NewAPI(WithSettingEngine(offerSE)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	answerPC, err := NewAPI(WithSettingEngine(answerSE)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	offerSender := &stagedCandidateSender{remote: answerPC}
	answerSender := &stagedCandidateSender{remote: offerPC}

	offerPC.OnICECandidate(func(c *ICECandidate) {
		if c == nil {
			return
		}

		switch c.Address {
		case primaryOfferIP:
			offerSender.addCandidate(makeSrflxCandidateInit(*c), true)
			host := *c
			host.Priority = 1
			offerSender.addCandidate(host.ToJSON(), false)
		case secondaryOfferIP:
			host := *c
			host.Priority = 1
			offerSender.addCandidate(host.ToJSON(), false)
		}
	})

	answerPC.OnICECandidate(func(c *ICECandidate) {
		if c == nil {
			return
		}

		switch c.Address {
		case primaryAnswerIP:
			answerSender.addCandidate(makeSrflxCandidateInit(*c), true)
			host := *c
			host.Priority = 1
			answerSender.addCandidate(host.ToJSON(), false)
		case secondaryAnswerIP:
			host := *c
			host.Priority = 1
			answerSender.addCandidate(host.ToJSON(), false)
		}
	})

	cleanup := func() {
		closePairNow(t, offerPC, answerPC)
		assert.NoError(t, router.Stop())
	}

	return offerPC, answerPC, offerSender, answerSender, cleanup
}

func startTrickleRenomination(
	t *testing.T,
	offerPC, answerPC *PeerConnection,
	offerSender, answerSender *stagedCandidateSender,
) {
	t.Helper()

	_, err := offerPC.CreateDataChannel("renomination-data", nil)
	assert.NoError(t, err)

	offer, err := offerPC.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, offerPC.SetLocalDescription(offer))
	assert.NoError(t, answerPC.SetRemoteDescription(offer))

	answer, err := answerPC.CreateAnswer(nil)
	assert.NoError(t, err)
	assert.NoError(t, answerPC.SetLocalDescription(answer))
	assert.NoError(t, offerPC.SetRemoteDescription(*answerPC.LocalDescription()))

	assert.NoError(t, offerSender.flushSrflx())
	assert.NoError(t, answerSender.flushSrflx())
}
