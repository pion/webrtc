package webrtc

import (
	"context"
	"errors"
	"sync"

	"github.com/pions/webrtc/pkg/ice"
)

// RTCIceTransport allows an application access to information about the ICE
// transport over which packets are sent and received.
type RTCIceTransport struct {
	lock sync.RWMutex

	role RTCIceRole
	// Component RTCIceComponent
	// State RTCIceTransportState
	// gatheringState RTCIceGathererState

	gatherer *RTCIceGatherer
	conn     *ice.Conn
}

// func (t *RTCIceTransport) GetLocalCandidates() []RTCIceCandidate {
//
// }
//
// func (t *RTCIceTransport) GetRemoteCandidates() []RTCIceCandidate {
//
// }
//
// func (t *RTCIceTransport) GetSelectedCandidatePair() RTCIceCandidatePair {
//
// }
//
// func (t *RTCIceTransport) GetLocalParameters() RTCIceParameters {
//
// }
//
// func (t *RTCIceTransport) GetRemoteParameters() RTCIceParameters {
//
// }

// NewRTCIceTransport creates a new NewRTCIceTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func NewRTCIceTransport(gatherer *RTCIceGatherer) *RTCIceTransport {
	return &RTCIceTransport{gatherer: gatherer}
}

// Start incoming connectivity checks based on its configured role.
func (t *RTCIceTransport) Start(gatherer *RTCIceGatherer, params RTCIceParameters, role *RTCIceRole) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if gatherer != nil {
		t.gatherer = gatherer
	}

	if err := t.ensureGatherer(); err != nil {
		return err
	}

	if role == nil {
		controlled := RTCIceRoleControlled
		role = &controlled
	}
	t.role = *role

	switch t.role {
	case RTCIceRoleControlling:
		iceConn, err := t.gatherer.agent.Dial(context.TODO(),
			params.UsernameFragment,
			params.Password)
		if err != nil {
			return err
		}
		t.conn = iceConn

	case RTCIceRoleControlled:
		iceConn, err := t.gatherer.agent.Accept(context.TODO(),
			params.UsernameFragment,
			params.Password)
		if err != nil {
			return err
		}
		t.conn = iceConn

	default:
		return errors.New("Unknown ICE Role")
	}

	return nil
}

// Role indicates the current role of the ICE transport.
func (t *RTCIceTransport) Role() RTCIceRole {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.role
}

// SetRemoteCandidates sets the sequence of candidates associated with the remote RTCIceTransport.
func (t *RTCIceTransport) SetRemoteCandidates(remoteCandidates []RTCIceCandidate) error {
	t.lock.RLock()
	defer t.lock.RUnlock()

	if err := t.ensureGatherer(); err != nil {
		return err
	}

	for _, c := range remoteCandidates {
		i, err := c.toICE()
		if err != nil {
			return err
		}
		err = t.gatherer.agent.AddRemoteCandidate(i)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *RTCIceTransport) ensureGatherer() error {
	if t.gatherer == nil ||
		t.gatherer.agent == nil {
		return errors.New("Gatherer not started")
	}

	return nil
}
