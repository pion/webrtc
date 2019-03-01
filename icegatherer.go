package webrtc

import (
	"errors"
	"sync"

	"github.com/pions/webrtc/internal/ice"
)

// The ICEGatherer gathers local host, server reflexive and relay
// candidates, as well as enabling the retrieval of local Interactive
// Connectivity Establishment (ICE) parameters which can be
// exchanged in signaling.
type ICEGatherer struct {
	lock  sync.RWMutex
	state ICEGathererState

	validatedServers []*ice.URL

	agent *ice.Agent

	api *API
}

// NewICEGatherer creates a new NewICEGatherer.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewICEGatherer(opts ICEGatherOptions) (*ICEGatherer, error) {
	validatedServers := []*ice.URL{}
	if len(opts.ICEServers) > 0 {
		for _, server := range opts.ICEServers {
			url, err := server.validate()
			if err != nil {
				return nil, err
			}
			validatedServers = append(validatedServers, url...)
		}
	}

	return &ICEGatherer{
		state:            ICEGathererStateNew,
		validatedServers: validatedServers,
		api:              api,
	}, nil
}

// State indicates the current state of the ICE gatherer.
func (g *ICEGatherer) State() ICEGathererState {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.state
}

// Gather ICE candidates.
func (g *ICEGatherer) Gather() error {
	g.lock.Lock()
	defer g.lock.Unlock()

	config := &ice.AgentConfig{
		Urls:              g.validatedServers,
		PortMin:           g.api.settingEngine.ephemeralUDP.PortMin,
		PortMax:           g.api.settingEngine.ephemeralUDP.PortMax,
		ConnectionTimeout: g.api.settingEngine.timeout.ICEConnection,
		KeepaliveInterval: g.api.settingEngine.timeout.ICEKeepalive,
	}

	agent, err := ice.NewAgent(config)
	if err != nil {
		return err
	}

	g.agent = agent
	g.state = ICEGathererStateComplete

	return nil
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
	g.lock.RLock()
	defer g.lock.RUnlock()
	if g.agent == nil {
		return ICEParameters{}, errors.New("gatherer not started")
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
	g.lock.RLock()
	defer g.lock.RUnlock()

	if g.agent == nil {
		return nil, errors.New("gatherer not started")
	}

	iceCandidates, err := g.agent.GetLocalCandidates()
	if err != nil {
		return nil, err
	}

	return newICECandidatesFromICE(iceCandidates)
}
