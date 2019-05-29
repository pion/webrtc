// +build !js

package webrtc

import (
	"errors"
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
	agent          *ice.Agent

	portMin           uint16
	portMax           uint16
	candidateTypes    []ice.CandidateType
	connectionTimeout *time.Duration
	keepaliveInterval *time.Duration
	loggerFactory     logging.LoggerFactory
	log               logging.LeveledLogger
	networkTypes      []NetworkType

	onLocalCandidateHdlr func(candidate *ICECandidate)
	onStateChangeHdlr    func(state ICEGathererState)
}

// NewICEGatherer creates a new NewICEGatherer.
func NewICEGatherer(
	portMin uint16,
	portMax uint16,
	connectionTimeout *time.Duration,
	keepaliveInterval *time.Duration,
	loggerFactory logging.LoggerFactory,
	networkTypes []NetworkType,
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
	if opts.ICEGatherPolicy == ICETransportPolicyRelay {
		candidateTypes = append(candidateTypes, ice.CandidateTypeRelay)
	}

	return &ICEGatherer{
		state:             ICEGathererStateNew,
		validatedServers:  validatedServers,
		portMin:           portMin,
		portMax:           portMax,
		connectionTimeout: connectionTimeout,
		keepaliveInterval: keepaliveInterval,
		loggerFactory:     loggerFactory,
		log:               loggerFactory.NewLogger("ice"),
		networkTypes:      networkTypes,
		candidateTypes:    candidateTypes,
	}, nil
}

func (g *ICEGatherer) createAgent() error {
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
		Trickle:           agentIsTrickle,
		Urls:              g.validatedServers,
		PortMin:           g.portMin,
		PortMax:           g.portMax,
		ConnectionTimeout: g.connectionTimeout,
		KeepaliveInterval: g.keepaliveInterval,
		LoggerFactory:     g.loggerFactory,
		CandidateTypes:    g.candidateTypes,
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
	if agentIsTrickle {
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
