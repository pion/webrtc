package webrtc

import (
	"context"
	"errors"
	"sync"

	"github.com/pions/webrtc/internal/mux"
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

	onConnectionStateChangeHdlr func(RTCIceTransportState)

	gatherer *RTCIceGatherer
	conn     *ice.Conn
	mux      *mux.Mux
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
func (api *API) NewRTCIceTransport(gatherer *RTCIceGatherer) *RTCIceTransport {
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

	agent := t.gatherer.agent
	err := agent.OnConnectionStateChange(func(iceState ice.ConnectionState) {
		t.onConnectionStateChange(newRTCIceTransportStateFromICE(iceState))
	})
	if err != nil {
		return err
	}

	if role == nil {
		controlled := RTCIceRoleControlled
		role = &controlled
	}
	t.role = *role

	// Drop the lock here to allow trickle-ICE candidates to be
	// added so that the agent can complete a connection
	t.lock.Unlock()

	var iceConn *ice.Conn
	switch *role {
	case RTCIceRoleControlling:
		iceConn, err = agent.Dial(context.TODO(),
			params.UsernameFragment,
			params.Password)

	case RTCIceRoleControlled:
		iceConn, err = agent.Accept(context.TODO(),
			params.UsernameFragment,
			params.Password)

	default:
		err = errors.New("Unknown ICE Role")
	}

	// Reacquire the lock to set the connection/mux
	t.lock.Lock()
	if err != nil {
		return err
	}

	t.conn = iceConn
	t.mux = mux.NewMux(t.conn, receiveMTU)

	return nil
}

// Stop irreversibly stops the RTCIceTransport.
func (t *RTCIceTransport) Stop() error {
	// Close the Mux. This closes the Mux and the underlying ICE conn.
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.mux != nil {
		return t.mux.Close()
	}
	return nil
}

// OnConnectionStateChange sets a handler that is fired when the ICE
// connection state changes.
func (t *RTCIceTransport) OnConnectionStateChange(f func(RTCIceTransportState)) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.onConnectionStateChangeHdlr = f
}

func (t *RTCIceTransport) onConnectionStateChange(state RTCIceTransportState) {
	t.lock.RLock()
	hdlr := t.onConnectionStateChangeHdlr
	t.lock.RUnlock()
	if hdlr != nil {
		hdlr(state)
	}
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

// AddRemoteCandidate adds a candidate associated with the remote RTCIceTransport.
func (t *RTCIceTransport) AddRemoteCandidate(remoteCandidate RTCIceCandidate) error {
	t.lock.RLock()
	defer t.lock.RUnlock()

	if err := t.ensureGatherer(); err != nil {
		return err
	}

	c, err := remoteCandidate.toICE()
	if err != nil {
		return err
	}
	err = t.gatherer.agent.AddRemoteCandidate(c)
	if err != nil {
		return err
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
