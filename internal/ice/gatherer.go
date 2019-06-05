// +build !js

package ice

import (
	"errors"
	"sync"
	"time"

	"github.com/pion/ice"
	"github.com/pion/logging"
)

// Gatherer gathers local host, server reflexive and relay
// candidates, as well as enabling the retrieval of local Interactive
// Connectivity Establishment (ICE) parameters which can be
// exchanged in signaling.
type Gatherer struct {
	lock  sync.RWMutex
	state GathererState

	validatedServers []*ice.URL

	agentIsTrickle bool
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

	onLocalCandidateHdlr func(candidate *Candidate)
	onStateChangeHdlr    func(state GathererState)
}

// NewGatherer creates a new Gatherer.
func NewGatherer(
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
	networkTypes []NetworkType,
	opts GatherOptions,
) (*Gatherer, error) {
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
	if opts.ICEGatherPolicy == TransportPolicyRelay {
		candidateTypes = append(candidateTypes, ice.CandidateTypeRelay)
	}

	return &Gatherer{
		state:                     GathererStateNew,
		validatedServers:          validatedServers,
		portMin:                   portMin,
		portMax:                   portMax,
		connectionTimeout:         connectionTimeout,
		keepaliveInterval:         keepaliveInterval,
		loggerFactory:             loggerFactory,
		log:                       loggerFactory.NewLogger("ice"),
		networkTypes:              networkTypes,
		candidateTypes:            candidateTypes,
		candidateSelectionTimeout: candidateSelectionTimeout,
		hostAcceptanceMinWait:     hostAcceptanceMinWait,
		srflxAcceptanceMinWait:    srflxAcceptanceMinWait,
		prflxAcceptanceMinWait:    prflxAcceptanceMinWait,
		relayAcceptanceMinWait:    relayAcceptanceMinWait,
	}, nil
}

func (g *Gatherer) createAgent() error {
	g.lock.Lock()
	defer g.lock.Unlock()
	agentIsTrickle := g.onLocalCandidateHdlr != nil || g.onStateChangeHdlr != nil

	if g.agent != nil {
		if !g.agentIsTrickle && agentIsTrickle {
			return errors.New("ICEAgent created without OnCandidate or StateChange handler, but now has one set")
		}

		return nil
	}

	config := &ice.AgentConfig{
		Trickle:                   agentIsTrickle,
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
	}

	requestedNetworkTypes := g.networkTypes
	if len(requestedNetworkTypes) == 0 {
		requestedNetworkTypes = supportedNetworkTypes
	}

	for _, typ := range requestedNetworkTypes {
		config.NetworkTypes = append(config.NetworkTypes, ice.NetworkType(typ))
	}

	agent, err := ice.NewAgent(config)
	if err != nil {
		return err
	}

	g.agent = agent
	g.agentIsTrickle = agentIsTrickle
	if !agentIsTrickle {
		g.state = GathererStateComplete
	}

	return nil
}

// Gather ICE candidates.
func (g *Gatherer) Gather() error {
	if err := g.createAgent(); err != nil {
		return err
	}

	g.lock.Lock()
	onLocalCandidateHdlr := g.onLocalCandidateHdlr
	isTrickle := g.agentIsTrickle
	agent := g.agent
	g.lock.Unlock()

	if !isTrickle {
		return nil
	}

	g.setState(GathererStateGathering)
	if err := agent.OnCandidate(func(candidate ice.Candidate) {
		if candidate != nil {
			c, err := newCandidateFromICE(candidate)
			if err != nil {
				g.log.Warnf("Failed to convert ice.Candidate: %s", err)
				return
			}
			onLocalCandidateHdlr(&c)
		} else {
			g.setState(GathererStateComplete)
			onLocalCandidateHdlr(nil)
		}
	}); err != nil {
		return err
	}
	return agent.GatherCandidates()
}

// Close prunes all local candidates, and closes the ports.
func (g *Gatherer) Close() error {
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

// GetLocalParameters returns the ICE parameters of the Gatherer.
func (g *Gatherer) GetLocalParameters() (Parameters, error) {
	if err := g.createAgent(); err != nil {
		return Parameters{}, err
	}

	frag, pwd := g.agent.GetLocalUserCredentials()
	return Parameters{
		UsernameFragment: frag,
		Password:         pwd,
		ICELite:          false,
	}, nil
}

// GetLocalCandidates returns the sequence of valid local candidates associated with the Gatherer.
func (g *Gatherer) GetLocalCandidates() ([]Candidate, error) {
	if err := g.createAgent(); err != nil {
		return nil, err
	}
	iceCandidates, err := g.agent.GetLocalCandidates()
	if err != nil {
		return nil, err
	}

	return newCandidatesFromICE(iceCandidates)
}

// OnLocalCandidate sets an event handler which fires when a new local ICE candidate is available
func (g *Gatherer) OnLocalCandidate(f func(*Candidate)) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.onLocalCandidateHdlr = f
}

// OnStateChange fires any time the Gatherer changes
func (g *Gatherer) OnStateChange(f func(GathererState)) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.onStateChangeHdlr = f
}

// State indicates the current state of the ICE gatherer.
func (g *Gatherer) State() GathererState {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.state
}

func (g *Gatherer) setState(s GathererState) {
	g.lock.Lock()
	g.state = s
	hdlr := g.onStateChangeHdlr
	g.lock.Unlock()

	if hdlr != nil {
		go hdlr(s)
	}
}

func (g *Gatherer) getAgent() *ice.Agent {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.agent
}

// AgentIsTrickle returns true if agent is in trickle mode.
func (g *Gatherer) AgentIsTrickle() bool {
	return g.agentIsTrickle
}
