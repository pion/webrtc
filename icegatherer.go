// +build !js

package webrtc

import (
	"sync"
	"time"

	"github.com/pion/ice"
	"github.com/pion/logging"
)

// ICEGatherer gathers local host, server reflexive and relay
// candidates, as well as enabling the retrieval of local Interactive
// Connectivity Establishment (ICE) parameters which can be
// exchanged in signaling.
type ICEGatherer struct {
	lock  sync.RWMutex
	state ICEGathererState

	validatedServers []*ice.URL

	agentIsTrickle bool
	lite           bool
	agent          *ice.Agent

	portMin                   uint16
	portMax                   uint16
	candidateTypes            []ice.CandidateType
	connectionTimeout         *time.Duration
	keepaliveInterval         *time.Duration
	candidateSelectionTimeout *time.Duration
	hostAcceptanceMinWait     *time.Duration
	srflxAcceptanceMinWait    *time.Duration
	prflxAcceptanceMinWait    *time.Duration
	relayAcceptanceMinWait    *time.Duration
	loggerFactory             logging.LoggerFactory
	log                       logging.LeveledLogger
	networkTypes              []NetworkType
	interfaceFilter           func(string) bool
	nat1To1IPs                []string
	nat1To1IPCandidateType    ice.CandidateType

	onLocalCandidateHdlr func(candidate *ICECandidate)
	onStateChangeHdlr    func(state ICEGathererState)
}

// NewICEGatherer creates a new NewICEGatherer.
func NewICEGatherer(
	portMin uint16,
	portMax uint16,
	connectionTimeout,
	keepaliveInterval,
	candidateSelectionTimeout,
	hostAcceptanceMinWait,
	srflxAcceptanceMinWait,
	prflxAcceptanceMinWait,
	relayAcceptanceMinWait *time.Duration,
	loggerFactory logging.LoggerFactory,
	agentIsTrickle bool,
	lite bool,
	networkTypes []NetworkType,
	interfaceFilter func(string) bool,
	nat1To1IPs []string,
	nat1To1IPCandidateType ICECandidateType,
	opts ICEGatherOptions,
) (*ICEGatherer, error) {
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

	candidateTypes := []ice.CandidateType{}
	if lite {
		candidateTypes = append(candidateTypes, ice.CandidateTypeHost)
	} else if opts.ICEGatherPolicy == ICETransportPolicyRelay {
		candidateTypes = append(candidateTypes, ice.CandidateTypeRelay)
	}

	var nat1To1CandiTyp ice.CandidateType
	switch nat1To1IPCandidateType {
	case ICECandidateTypeHost:
		nat1To1CandiTyp = ice.CandidateTypeHost
	case ICECandidateTypeSrflx:
		nat1To1CandiTyp = ice.CandidateTypeServerReflexive
	default:
		nat1To1CandiTyp = ice.CandidateTypeUnspecified
	}

	return &ICEGatherer{
		state:                     ICEGathererStateNew,
		validatedServers:          validatedServers,
		portMin:                   portMin,
		portMax:                   portMax,
		connectionTimeout:         connectionTimeout,
		keepaliveInterval:         keepaliveInterval,
		loggerFactory:             loggerFactory,
		log:                       loggerFactory.NewLogger("ice"),
		agentIsTrickle:            agentIsTrickle,
		lite:                      lite,
		networkTypes:              networkTypes,
		candidateTypes:            candidateTypes,
		candidateSelectionTimeout: candidateSelectionTimeout,
		hostAcceptanceMinWait:     hostAcceptanceMinWait,
		srflxAcceptanceMinWait:    srflxAcceptanceMinWait,
		prflxAcceptanceMinWait:    prflxAcceptanceMinWait,
		relayAcceptanceMinWait:    relayAcceptanceMinWait,
		interfaceFilter:           interfaceFilter,
		nat1To1IPs:                nat1To1IPs,
		nat1To1IPCandidateType:    nat1To1CandiTyp,
	}, nil
}

func (g *ICEGatherer) createAgent() error {
	g.lock.Lock()
	defer g.lock.Unlock()

	if g.agent != nil {
		return nil
	}

	config := &ice.AgentConfig{
		Trickle:                   g.agentIsTrickle,
		Lite:                      g.lite,
		Urls:                      g.validatedServers,
		PortMin:                   g.portMin,
		PortMax:                   g.portMax,
		ConnectionTimeout:         g.connectionTimeout,
		KeepaliveInterval:         g.keepaliveInterval,
		LoggerFactory:             g.loggerFactory,
		CandidateTypes:            g.candidateTypes,
		CandidateSelectionTimeout: g.candidateSelectionTimeout,
		HostAcceptanceMinWait:     g.hostAcceptanceMinWait,
		SrflxAcceptanceMinWait:    g.srflxAcceptanceMinWait,
		PrflxAcceptanceMinWait:    g.prflxAcceptanceMinWait,
		RelayAcceptanceMinWait:    g.relayAcceptanceMinWait,
		InterfaceFilter:           g.interfaceFilter,
	}

	requestedNetworkTypes := g.networkTypes
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
	if !g.agentIsTrickle {
		g.state = ICEGathererStateComplete
	}

	return nil
}

// Gather ICE candidates.
func (g *ICEGatherer) Gather() error {
	if err := g.createAgent(); err != nil {
		return err
	}

	g.lock.Lock()
	onLocalCandidateHdlr := g.onLocalCandidateHdlr
	if onLocalCandidateHdlr == nil {
		onLocalCandidateHdlr = func(*ICECandidate) {}
	}

	isTrickle := g.agentIsTrickle
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
	}

	err := g.agent.Close()
	if err != nil {
		return err
	}
	g.agent = nil

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
func (g *ICEGatherer) OnLocalCandidate(f func(*ICECandidate)) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.onLocalCandidateHdlr = f
}

// OnStateChange fires any time the ICEGatherer changes
func (g *ICEGatherer) OnStateChange(f func(ICEGathererState)) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.onStateChangeHdlr = f
}

// State indicates the current state of the ICE gatherer.
func (g *ICEGatherer) State() ICEGathererState {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.state
}

func (g *ICEGatherer) setState(s ICEGathererState) {
	g.lock.Lock()
	g.state = s
	hdlr := g.onStateChangeHdlr
	g.lock.Unlock()

	if hdlr != nil {
		go hdlr(s)
	}
}

func (g *ICEGatherer) getAgent() *ice.Agent {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.agent
}

// SignalCandidates imitates gathering process to backward support old tricle
// false behavior.
func (g *ICEGatherer) SignalCandidates() error {
	candidates, err := g.GetLocalCandidates()
	if err != nil {
		return err
	}

	g.lock.Lock()
	onLocalCandidateHdlr := g.onLocalCandidateHdlr
	g.lock.Unlock()

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
	collector.Collecting()

	go func(collector *statsReportCollector) {
		for _, candidatePairStats := range g.agent.GetCandidatePairsStats() {
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

		for _, candidateStats := range g.agent.GetLocalCandidatesStats() {
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

		for _, candidateStats := range g.agent.GetRemoteCandidatesStats() {
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
	}(collector)
}
