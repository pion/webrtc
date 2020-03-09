// +build !js

package webrtc

import (
	"sync"
	"sync/atomic"

	"github.com/pion/ice"
	"github.com/pion/logging"
)

// ICEGatherer gathers local host, server reflexive and relay
// candidates, as well as enabling the retrieval of local Interactive
// Connectivity Establishment (ICE) parameters which can be
// exchanged in signaling.
type ICEGatherer struct {
	lock  sync.RWMutex
	log   logging.LeveledLogger
	state ICEGathererState

	validatedServers []*ice.URL
	gatherPolicy     ICETransportPolicy

	agent *ice.Agent

	onLocalCandidateHdlr atomic.Value // func(candidate *ICECandidate)
	onStateChangeHdlr    atomic.Value // func(state ICEGathererState)

	api *API
}

// NewICEGatherer creates a new NewICEGatherer.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewICEGatherer(opts ICEGatherOptions) (*ICEGatherer, error) {
	var validatedServers []*ice.URL
	if len(opts.ICEServers) > 0 {
		for _, server := range opts.ICEServers {
			url, err := server.urls()
			if err != nil {
				return nil, err
			}
			validatedServers = append(validatedServers, url...)
		}
	}

	return &ICEGatherer{
		state:            ICEGathererStateNew,
		gatherPolicy:     opts.ICEGatherPolicy,
		validatedServers: validatedServers,
		api:              api,
		log:              api.settingEngine.LoggerFactory.NewLogger("ice"),
	}, nil
}

func (g *ICEGatherer) createAgent() error {
	g.lock.Lock()
	defer g.lock.Unlock()

	if g.agent != nil {
		return nil
	}

	candidateTypes := []ice.CandidateType{}
	if g.api.settingEngine.candidates.ICELite {
		candidateTypes = append(candidateTypes, ice.CandidateTypeHost)
	} else if g.gatherPolicy == ICETransportPolicyRelay {
		candidateTypes = append(candidateTypes, ice.CandidateTypeRelay)
	}

	var nat1To1CandiTyp ice.CandidateType
	switch g.api.settingEngine.candidates.NAT1To1IPCandidateType {
	case ICECandidateTypeHost:
		nat1To1CandiTyp = ice.CandidateTypeHost
	case ICECandidateTypeSrflx:
		nat1To1CandiTyp = ice.CandidateTypeServerReflexive
	default:
		nat1To1CandiTyp = ice.CandidateTypeUnspecified
	}

	var multicastDNSMode ice.MulticastDNSMode
	if g.api.settingEngine.candidates.GenerateMulticastDNSCandidates {
		multicastDNSMode = ice.MulticastDNSModeQueryAndGather
	}

	config := &ice.AgentConfig{
		Trickle:                   g.api.settingEngine.candidates.ICETrickle,
		Lite:                      g.api.settingEngine.candidates.ICELite,
		Urls:                      g.validatedServers,
		PortMin:                   g.api.settingEngine.ephemeralUDP.PortMin,
		PortMax:                   g.api.settingEngine.ephemeralUDP.PortMax,
		ConnectionTimeout:         g.api.settingEngine.timeout.ICEConnection,
		KeepaliveInterval:         g.api.settingEngine.timeout.ICEKeepalive,
		LoggerFactory:             g.api.settingEngine.LoggerFactory,
		CandidateTypes:            candidateTypes,
		CandidateSelectionTimeout: g.api.settingEngine.timeout.ICECandidateSelectionTimeout,
		HostAcceptanceMinWait:     g.api.settingEngine.timeout.ICEHostAcceptanceMinWait,
		SrflxAcceptanceMinWait:    g.api.settingEngine.timeout.ICESrflxAcceptanceMinWait,
		PrflxAcceptanceMinWait:    g.api.settingEngine.timeout.ICEPrflxAcceptanceMinWait,
		RelayAcceptanceMinWait:    g.api.settingEngine.timeout.ICERelayAcceptanceMinWait,
		InterfaceFilter:           g.api.settingEngine.candidates.InterfaceFilter,
		NAT1To1IPs:                g.api.settingEngine.candidates.NAT1To1IPs,
		NAT1To1IPCandidateType:    nat1To1CandiTyp,
		Net:                       g.api.settingEngine.vnet,
		MulticastDNSMode:          multicastDNSMode,
		MulticastDNSHostName:      g.api.settingEngine.candidates.MulticastDNSHostName,
		LocalUfrag:                g.api.settingEngine.candidates.UsernameFragment,
		LocalPwd:                  g.api.settingEngine.candidates.Password,
	}

	requestedNetworkTypes := g.api.settingEngine.candidates.ICENetworkTypes
	if len(requestedNetworkTypes) == 0 {
		requestedNetworkTypes = supportedNetworkTypes()
	}

	for _, typ := range requestedNetworkTypes {
		config.NetworkTypes = append(config.NetworkTypes, ice.NetworkType(typ))
	}

	agent, err := ice.NewAgent(config)
	if err != nil {
		return err
	}

	g.agent = agent
	if !g.api.settingEngine.candidates.ICETrickle {
		atomicStoreICEGathererState(&g.state, ICEGathererStateComplete)
	}

	return nil
}

// Gather ICE candidates.
func (g *ICEGatherer) Gather() error {
	if err := g.createAgent(); err != nil {
		return err
	}

	onLocalCandidateHdlr := func(*ICECandidate) {}
	if hdlr, ok := g.onLocalCandidateHdlr.Load().(func(candidate *ICECandidate)); ok && hdlr != nil {
		onLocalCandidateHdlr = hdlr
	}

	g.lock.Lock()
	isTrickle := g.api.settingEngine.candidates.ICETrickle
	agent := g.agent
	g.lock.Unlock()

	if !isTrickle {
		return nil
	}

	g.setState(ICEGathererStateGathering)
	if err := agent.OnCandidate(func(candidate ice.Candidate) {
		if candidate != nil {
			c, err := newICECandidateFromICE(candidate)
			if err != nil {
				g.log.Warnf("Failed to convert ice.Candidate: %s", err)
				return
			}
			onLocalCandidateHdlr(&c)
		} else {
			g.setState(ICEGathererStateComplete)

			onLocalCandidateHdlr(nil)
		}
	}); err != nil {
		return err
	}
	return agent.GatherCandidates()
}

// Close prunes all local candidates, and closes the ports.
func (g *ICEGatherer) Close() error {
	g.lock.Lock()
	defer g.lock.Unlock()

	if g.agent == nil {
		return nil
	} else if err := g.agent.Close(); err != nil {
		return err
	}

	g.agent = nil
	g.setState(ICEGathererStateClosed)

	return nil
}

// GetLocalParameters returns the ICE parameters of the ICEGatherer.
func (g *ICEGatherer) GetLocalParameters() (ICEParameters, error) {
	if err := g.createAgent(); err != nil {
		return ICEParameters{}, err
	}

	frag, pwd := g.agent.GetLocalUserCredentials()
	return ICEParameters{
		UsernameFragment: frag,
		Password:         pwd,
		ICELite:          false,
	}, nil
}

// GetLocalCandidates returns the sequence of valid local candidates associated with the ICEGatherer.
func (g *ICEGatherer) GetLocalCandidates() ([]ICECandidate, error) {
	if err := g.createAgent(); err != nil {
		return nil, err
	}
	iceCandidates, err := g.agent.GetLocalCandidates()
	if err != nil {
		return nil, err
	}

	return newICECandidatesFromICE(iceCandidates)
}

// OnLocalCandidate sets an event handler which fires when a new local ICE candidate is available
// Take note that the handler is gonna be called with a nil pointer when gathering is finished.
func (g *ICEGatherer) OnLocalCandidate(f func(*ICECandidate)) {
	g.onLocalCandidateHdlr.Store(f)
}

// OnStateChange fires any time the ICEGatherer changes
func (g *ICEGatherer) OnStateChange(f func(ICEGathererState)) {
	g.onStateChangeHdlr.Store(f)
}

// State indicates the current state of the ICE gatherer.
func (g *ICEGatherer) State() ICEGathererState {
	return atomicLoadICEGathererState(&g.state)
}

func (g *ICEGatherer) setState(s ICEGathererState) {
	atomicStoreICEGathererState(&g.state, s)

	if hdlr, ok := g.onStateChangeHdlr.Load().(func(state ICEGathererState)); ok && hdlr != nil {
		hdlr(s)
	}
}

func (g *ICEGatherer) getAgent() *ice.Agent {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.agent
}

// SignalCandidates imitates gathering process to backward support old trickle
// false behavior.
func (g *ICEGatherer) SignalCandidates() error {
	candidates, err := g.GetLocalCandidates()
	if err != nil {
		return err
	}

	var onLocalCandidateHdlr func(*ICECandidate)
	if hdlr, ok := g.onLocalCandidateHdlr.Load().(func(candidate *ICECandidate)); ok {
		onLocalCandidateHdlr = hdlr
	}

	if onLocalCandidateHdlr != nil {
		go func() {
			for i := range candidates {
				onLocalCandidateHdlr(&candidates[i])
			}
			// Call the handler one last time with nil. This is a signal that candidate
			// gathering is complete.
			onLocalCandidateHdlr(nil)
		}()
	}
	return nil
}

func (g *ICEGatherer) collectStats(collector *statsReportCollector) {
	agent := g.getAgent()
	if agent == nil {
		return
	}

	collector.Collecting()
	go func(collector *statsReportCollector, agent *ice.Agent) {
		for _, candidatePairStats := range agent.GetCandidatePairsStats() {
			collector.Collecting()

			state, err := toStatsICECandidatePairState(candidatePairStats.State)
			if err != nil {
				g.log.Error(err.Error())
			}

			pairID := newICECandidatePairStatsID(candidatePairStats.LocalCandidateID,
				candidatePairStats.RemoteCandidateID)

			stats := ICECandidatePairStats{
				Timestamp: statsTimestampFrom(candidatePairStats.Timestamp),
				Type:      StatsTypeCandidatePair,
				ID:        pairID,
				// TransportID:
				LocalCandidateID:            candidatePairStats.LocalCandidateID,
				RemoteCandidateID:           candidatePairStats.RemoteCandidateID,
				State:                       state,
				Nominated:                   candidatePairStats.Nominated,
				PacketsSent:                 candidatePairStats.PacketsSent,
				PacketsReceived:             candidatePairStats.PacketsReceived,
				BytesSent:                   candidatePairStats.BytesSent,
				BytesReceived:               candidatePairStats.BytesReceived,
				LastPacketSentTimestamp:     statsTimestampFrom(candidatePairStats.LastPacketSentTimestamp),
				LastPacketReceivedTimestamp: statsTimestampFrom(candidatePairStats.LastPacketReceivedTimestamp),
				FirstRequestTimestamp:       statsTimestampFrom(candidatePairStats.FirstRequestTimestamp),
				LastRequestTimestamp:        statsTimestampFrom(candidatePairStats.LastRequestTimestamp),
				LastResponseTimestamp:       statsTimestampFrom(candidatePairStats.LastResponseTimestamp),
				TotalRoundTripTime:          candidatePairStats.TotalRoundTripTime,
				CurrentRoundTripTime:        candidatePairStats.CurrentRoundTripTime,
				AvailableOutgoingBitrate:    candidatePairStats.AvailableOutgoingBitrate,
				AvailableIncomingBitrate:    candidatePairStats.AvailableIncomingBitrate,
				CircuitBreakerTriggerCount:  candidatePairStats.CircuitBreakerTriggerCount,
				RequestsReceived:            candidatePairStats.RequestsReceived,
				RequestsSent:                candidatePairStats.RequestsSent,
				ResponsesReceived:           candidatePairStats.ResponsesReceived,
				ResponsesSent:               candidatePairStats.ResponsesSent,
				RetransmissionsReceived:     candidatePairStats.RetransmissionsReceived,
				RetransmissionsSent:         candidatePairStats.RetransmissionsSent,
				ConsentRequestsSent:         candidatePairStats.ConsentRequestsSent,
				ConsentExpiredTimestamp:     statsTimestampFrom(candidatePairStats.ConsentExpiredTimestamp),
			}
			collector.Collect(stats.ID, stats)
		}

		for _, candidateStats := range agent.GetLocalCandidatesStats() {
			collector.Collecting()

			networkType, err := getNetworkType(candidateStats.NetworkType)
			if err != nil {
				g.log.Error(err.Error())
			}

			candidateType, err := getCandidateType(candidateStats.CandidateType)
			if err != nil {
				g.log.Error(err.Error())
			}

			stats := ICECandidateStats{
				Timestamp:     statsTimestampFrom(candidateStats.Timestamp),
				ID:            candidateStats.ID,
				Type:          StatsTypeLocalCandidate,
				NetworkType:   networkType,
				IP:            candidateStats.IP,
				Port:          int32(candidateStats.Port),
				Protocol:      networkType.Protocol(),
				CandidateType: candidateType,
				Priority:      int32(candidateStats.Priority),
				URL:           candidateStats.URL,
				RelayProtocol: candidateStats.RelayProtocol,
				Deleted:       candidateStats.Deleted,
			}
			collector.Collect(stats.ID, stats)
		}

		for _, candidateStats := range agent.GetRemoteCandidatesStats() {
			collector.Collecting()
			networkType, err := getNetworkType(candidateStats.NetworkType)
			if err != nil {
				g.log.Error(err.Error())
			}

			candidateType, err := getCandidateType(candidateStats.CandidateType)
			if err != nil {
				g.log.Error(err.Error())
			}

			stats := ICECandidateStats{
				Timestamp:     statsTimestampFrom(candidateStats.Timestamp),
				ID:            candidateStats.ID,
				Type:          StatsTypeRemoteCandidate,
				NetworkType:   networkType,
				IP:            candidateStats.IP,
				Port:          int32(candidateStats.Port),
				Protocol:      networkType.Protocol(),
				CandidateType: candidateType,
				Priority:      int32(candidateStats.Priority),
				URL:           candidateStats.URL,
				RelayProtocol: candidateStats.RelayProtocol,
			}
			collector.Collect(stats.ID, stats)
		}
		collector.Done()
	}(collector, agent)
}
