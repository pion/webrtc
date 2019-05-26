// +build !js

package ice

import (
	"context"
	"errors"
	"sync"

	"github.com/pion/ice"
	"github.com/pion/logging"
	"github.com/pion/webrtc/v2/internal/mux"
)

// Transport allows an application access to information about the ICE
// transport over which packets are sent and received.
type Transport struct {
	lock sync.RWMutex

	role Role
	// Component Component
	// State TransportState
	// gatheringState GathererState

	onConnectionStateChangeHdlr       func(TransportState)
	onSelectedCandidatePairChangeHdlr func(*CandidatePair)

	state TransportState

	gatherer *Gatherer
	conn     *ice.Conn
	mux      *mux.Mux

	loggerFactory logging.LoggerFactory

	log logging.LeveledLogger
}

// func (t *Transport) GetLocalCandidates() []Candidate {
//
// }
//
// func (t *Transport) GetRemoteCandidates() []Candidate {
//
// }
//
// func (t *Transport) GetSelectedCandidatePair() CandidatePair {
//
// }
//
// func (t *Transport) GetLocalParameters() Parameters {
//
// }
//
// func (t *Transport) GetRemoteParameters() Parameters {
//
// }

// NewTransport creates a new NewTransport.
func NewTransport(gatherer *Gatherer, loggerFactory logging.LoggerFactory) *Transport {
	return &Transport{
		gatherer:      gatherer,
		loggerFactory: loggerFactory,
		log:           loggerFactory.NewLogger("ortc"),
		state:         TransportStateNew,
	}
}

// Start incoming connectivity checks based on its configured role.
func (t *Transport) Start(gatherer *Gatherer, params Parameters, role *Role) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if gatherer != nil {
		t.gatherer = gatherer
	}

	if err := t.ensureGatherer(); err != nil {
		return err
	}

	agent := t.gatherer.agent
	if err := agent.OnConnectionStateChange(func(iceState ice.ConnectionState) {
		state := newTransportStateFromICE(iceState)
		t.lock.Lock()
		t.state = state
		t.lock.Unlock()

		t.onConnectionStateChange(state)
	}); err != nil {
		return err
	}
	if err := agent.OnSelectedCandidatePairChange(func(local, remote ice.Candidate) {
		candidates, err := newCandidatesFromICE([]ice.Candidate{local, remote})
		if err != nil {
			t.log.Warnf("Unable to convert ICE candidates to ICECandidates: %s", err)
			return
		}
		t.onSelectedCandidatePairChange(NewCandidatePair(&candidates[0], &candidates[1]))
	}); err != nil {
		return err
	}

	if role == nil {
		controlled := RoleControlled
		role = &controlled
	}
	t.role = *role

	// Drop the lock here to allow trickle-ICE candidates to be
	// added so that the agent can complete a connection
	t.lock.Unlock()

	var iceConn *ice.Conn
	var err error
	switch *role {
	case RoleControlling:
		iceConn, err = agent.Dial(context.TODO(),
			params.UsernameFragment,
			params.Password)

	case RoleControlled:
		iceConn, err = agent.Accept(context.TODO(),
			params.UsernameFragment,
			params.Password)

	default:
		err = errors.New("unknown ICE Role")
	}

	// Reacquire the lock to set the connection/mux
	t.lock.Lock()
	if err != nil {
		return err
	}

	t.conn = iceConn

	config := mux.Config{
		Conn:          t.conn,
		BufferSize:    receiveMTU,
		LoggerFactory: t.loggerFactory,
	}
	t.mux = mux.NewMux(config)

	return nil
}

// Stop irreversibly stops the Transport.
func (t *Transport) Stop() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.mux != nil {
		return t.mux.Close()
	} else if t.gatherer != nil {
		return t.gatherer.Close()
	}
	return nil
}

// OnSelectedCandidatePairChange sets a handler that is invoked when a new
// ICE candidate pair is selected
func (t *Transport) OnSelectedCandidatePairChange(f func(*CandidatePair)) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.onSelectedCandidatePairChangeHdlr = f
}

func (t *Transport) onSelectedCandidatePairChange(pair *CandidatePair) {
	t.lock.RLock()
	hdlr := t.onSelectedCandidatePairChangeHdlr
	t.lock.RUnlock()
	if hdlr != nil {
		hdlr(pair)
	}
}

// OnConnectionStateChange sets a handler that is fired when the ICE
// connection state changes.
func (t *Transport) OnConnectionStateChange(f func(TransportState)) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.onConnectionStateChangeHdlr = f
}

func (t *Transport) onConnectionStateChange(state TransportState) {
	t.lock.RLock()
	hdlr := t.onConnectionStateChangeHdlr
	t.lock.RUnlock()
	if hdlr != nil {
		hdlr(state)
	}
}

// Role indicates the current role of the ICE transport.
func (t *Transport) Role() Role {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.role
}

// SetRemoteCandidates sets the sequence of candidates associated with the remote Transport.
func (t *Transport) SetRemoteCandidates(remoteCandidates []Candidate) error {
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

// AddRemoteCandidate adds a candidate associated with the remote Transport.
func (t *Transport) AddRemoteCandidate(remoteCandidate Candidate) error {
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

// State returns the current ice transport state.
func (t *Transport) State() TransportState {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.state
}

// NewEndpoint registers a new endpoint on the underlying mux.
func (t *Transport) NewEndpoint(f mux.MatchFunc) *mux.Endpoint {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.mux.NewEndpoint(f)
}

func (t *Transport) ensureGatherer() error {
	if t.gatherer == nil || t.gatherer.getAgent() == nil {
		return errors.New("gatherer not started")
	}

	return nil
}
