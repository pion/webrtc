package webrtc

import (
	"errors"
	"sync"

	"github.com/pions/webrtc/pkg/ice"
)

// The RTCIceGatherer gathers local host, server reflexive and relay
// candidates, as well as enabling the retrieval of local Interactive
// Connectivity Establishment (ICE) parameters which can be
// exchanged in signaling.
type RTCIceGatherer struct {
	lock  sync.RWMutex
	state RTCIceGathererState

	validatedServers []*ice.URL

	agent *ice.Agent
}

// NewRTCIceGatherer creates a new NewRTCIceGatherer.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func NewRTCIceGatherer(opts RTCIceGatherOptions) (*RTCIceGatherer, error) {
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

	return &RTCIceGatherer{
		state:            RTCIceGathererStateNew,
		validatedServers: validatedServers,
	}, nil
}

// State indicates the current state of the ICE gatherer.
func (g *RTCIceGatherer) State() RTCIceGathererState {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.state
}

// Gather ICE candidates.
func (g *RTCIceGatherer) Gather() error {
	g.lock.Lock()
	defer g.lock.Unlock()

	config := &ice.AgentConfig{
		Urls:     g.validatedServers,
		Notifier: nil, // TODO
		PortMin:  defaultSettingEngine.EphemeralUDP.PortMin,
		PortMax:  defaultSettingEngine.EphemeralUDP.PortMax,
	}
	agent, err := ice.NewAgent(config)
	if err != nil {
		return err
	}

	g.agent = agent
	g.state = RTCIceGathererStateComplete

	return nil
}

// GetLocalParameters returns the ICE parameters of the RTCIceGatherer.
func (g *RTCIceGatherer) GetLocalParameters() (RTCIceParameters, error) {
	g.lock.RLock()
	defer g.lock.RUnlock()
	if g.agent == nil {
		return RTCIceParameters{}, errors.New("Gatherer not started")
	}

	frag, pwd := g.agent.GetLocalUserCredentials()

	return RTCIceParameters{
		UsernameFragment: frag,
		Password:         pwd,
		IceLite:          false,
	}, nil
}

// GetLocalCandidates returns the sequence of valid local candidates associated with the RTCIceGatherer.
func (g *RTCIceGatherer) GetLocalCandidates() ([]RTCIceCandidate, error) {
	g.lock.RLock()
	defer g.lock.RUnlock()

	if g.agent == nil {
		return nil, errors.New("Gatherer not started")
	}

	iceCandidates, err := g.agent.GetLocalCandidates()
	if err != nil {
		return nil, err
	}

	return newRTCIceCandidatesFromICE(iceCandidates)
}
