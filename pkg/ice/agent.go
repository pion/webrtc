// Package ice implements the Interactive Connectivity Establishment (ICE)
// protocol defined in rfc5245.
package ice

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/util"
	"github.com/pkg/errors"
)

// Unknown defines default public constant to use for "enum" like struct
// comparisons when no value was defined.
const Unknown = iota

// OutboundCallback is the user defined Callback that is called when ICE traffic needs to sent
type OutboundCallback func(raw []byte, local *stun.TransportAddr, remote *net.UDPAddr)

func newCandidatePair(local, remote Candidate) CandidatePair {
	return CandidatePair{
		remote: remote,
		local:  local,
	}
}

// CandidatePair represents a combination of a local and remote candidate
type CandidatePair struct {
	// lastUpdateTime ?
	remote Candidate
	local  Candidate
}

func (c CandidatePair) getAddrs() (local *stun.TransportAddr, remote *net.UDPAddr) {
	localIP := net.ParseIP(c.local.GetBase().Address)
	localPort := c.local.GetBase().Port

	switch c := c.local.(type) {
	case *CandidateSrflx:
		localIP = net.ParseIP(c.RemoteAddress)
		localPort = c.RemotePort
	}

	return &stun.TransportAddr{
			IP:   localIP,
			Port: localPort,
		}, &net.UDPAddr{
			IP:   net.ParseIP(c.remote.GetBase().Address),
			Port: c.remote.GetBase().Port,
		}
}

// Agent represents the ICE agent
type Agent struct {
	sync.RWMutex

	outboundCallback OutboundCallback
	iceNotifier      func(ConnectionState)

	tieBreaker      uint64
	connectionState ConnectionState
	gatheringState  GatheringState

	haveStarted   bool
	isControlling bool
	taskLoopChan  chan bool

	LocalUfrag      string
	LocalPwd        string
	LocalCandidates []Candidate

	remoteUfrag      string
	remotePwd        string
	remoteCandidates []Candidate

	selectedPair CandidatePair
	validPairs   []CandidatePair
}

const (
	// taskLoopInterval is the interval at which the agent performs checks
	taskLoopInterval = 2 * time.Second

	// keepaliveInterval used to keep candidates alive
	keepaliveInterval = 10 * time.Second

	// connectionTimeout used to declare a connection dead
	connectionTimeout = 30 * time.Second
)

// NewAgent creates a new Agent
func NewAgent(outboundCallback OutboundCallback, iceNotifier func(ConnectionState)) *Agent {
	return &Agent{
		outboundCallback: outboundCallback,
		iceNotifier:      iceNotifier,

		tieBreaker:      rand.New(rand.NewSource(time.Now().UnixNano())).Uint64(),
		gatheringState:  GatheringStateComplete, // TODO trickle-ice
		connectionState: ConnectionStateNew,

		LocalUfrag: util.RandSeq(16),
		LocalPwd:   util.RandSeq(32),
	}
}

// Start starts the agent
func (a *Agent) Start(isControlling bool, remoteUfrag, remotePwd string) error {
	a.Lock()
	defer a.Unlock()

	if a.haveStarted {
		return errors.Errorf("Attempted to start agent twice")
	} else if remoteUfrag == "" {
		return errors.Errorf("remoteUfrag is empty")
	} else if remotePwd == "" {
		return errors.Errorf("remotePwd is empty")
	}

	a.isControlling = isControlling
	a.remoteUfrag = remoteUfrag
	a.remotePwd = remotePwd

	go a.taskLoop()
	return nil
}

func (a *Agent) pingCandidate(local, remote Candidate) {
	var msg *stun.Message
	var err error

	// The controlling agent MUST include the USE-CANDIDATE attribute in
	// order to nominate a candidate pair (Section 8.1.1).  The controlled
	// agent MUST NOT include the USE-CANDIDATE attribute in a Binding
	// request.

	if a.isControlling {
		msg, err = stun.Build(stun.ClassRequest, stun.MethodBinding, stun.GenerateTransactionId(),
			&stun.Username{Username: a.remoteUfrag + ":" + a.LocalUfrag},
			&stun.UseCandidate{},
			&stun.IceControlling{TieBreaker: a.tieBreaker},
			&stun.Priority{Priority: uint32(local.GetBase().Priority(HostCandidatePreference, 1))},
			&stun.MessageIntegrity{
				Key: []byte(a.remotePwd),
			},
			&stun.Fingerprint{},
		)
	} else {
		msg, err = stun.Build(stun.ClassRequest, stun.MethodBinding, stun.GenerateTransactionId(),
			&stun.Username{Username: a.remoteUfrag + ":" + a.LocalUfrag},
			&stun.IceControlled{TieBreaker: a.tieBreaker},
			&stun.Priority{Priority: uint32(local.GetBase().Priority(HostCandidatePreference, 1))},
			&stun.MessageIntegrity{
				Key: []byte(a.remotePwd),
			},
			&stun.Fingerprint{},
		)
	}

	if err != nil {
		fmt.Println(err)
		return
	}

	a.sendSTUN(msg, local, remote)
}

// keepaliveCandidate sends a STUN Binding Indication to the remote candidate
func (a *Agent) keepaliveCandidate(local, remote Candidate) {
	msg, err := stun.Build(stun.ClassIndication, stun.MethodBinding, stun.GenerateTransactionId(), &stun.Fingerprint{})

	if err != nil {
		fmt.Println(err)
		return
	}

	a.sendSTUN(msg, local, remote)
}

func (a *Agent) sendSTUN(msg *stun.Message, local, remote Candidate) {
	a.outboundCallback(msg.Pack(), &stun.TransportAddr{
		IP:   net.ParseIP(local.GetBase().Address),
		Port: local.GetBase().Port,
	}, &net.UDPAddr{
		IP:   net.ParseIP(remote.GetBase().Address),
		Port: remote.GetBase().Port,
	})
	remote.GetBase().seen(true)
}

func (a *Agent) updateConnectionState(newState ConnectionState) {
	a.connectionState = newState
	// Call handler async since we may be holding the agent lock
	// and the handler may also require it
	go a.iceNotifier(a.connectionState)
}

func (a *Agent) setValidPair(local, remote Candidate, selected bool) {
	p := newCandidatePair(local, remote)

	if selected {
		a.selectedPair = p
		a.validPairs = nil
		// TODO: only set state to connected on selecting final pair?
		a.updateConnectionState(ConnectionStateConnected)
	} else {
		// keep track of pairs with succesfull bindings since any of them
		// can be used for communication until the final pair is selected:
		// https://tools.ietf.org/html/draft-ietf-ice-rfc5245bis-20#section-12
		a.validPairs = append(a.validPairs, p)
	}
}

func (a *Agent) taskLoop() {
	// TODO this should be dynamic, and grow when the connection is stable
	t := time.NewTicker(taskLoopInterval)
	a.updateConnectionState(ConnectionStateChecking)

	for {
		select {
		case <-t.C:
			a.Lock()
			if a.validateSelectedPair() {
				a.checkKeepalive()
			} else {
				a.pingAllCandidates()
			}
			a.Unlock()
		case <-a.taskLoopChan:
			t.Stop()
			return
		}
	}
}

// validateSelectedPair checks if the selected pair is (still) valid
// Note: the caller should hold the agent lock.
func (a *Agent) validateSelectedPair() bool {
	if a.selectedPair.remote == nil || a.selectedPair.local == nil {
		// Not valid since not selected
		return false
	}

	if time.Since(a.selectedPair.remote.GetBase().LastReceived) > connectionTimeout {
		a.selectedPair.remote = nil
		a.selectedPair.local = nil
		a.updateConnectionState(ConnectionStateDisconnected)
		return false
	}

	return true
}

// checkKeepalive sends STUN Binding Indications to the selected pair
// if no packet has been sent on that pair in the last keepaliveInterval
// Note: the caller should hold the agent lock.
func (a *Agent) checkKeepalive() {
	if a.selectedPair.remote == nil || a.selectedPair.local == nil {
		return
	}

	if time.Since(a.selectedPair.remote.GetBase().LastSent) > keepaliveInterval {
		a.keepaliveCandidate(a.selectedPair.local, a.selectedPair.remote)
	}
}

// pingAllCandidates sends STUN Binding Requests to all candidates
// Note: the caller should hold the agent lock.
func (a *Agent) pingAllCandidates() {
	for _, localCandidate := range a.LocalCandidates {
		for _, remoteCandidate := range a.remoteCandidates {
			a.pingCandidate(localCandidate, remoteCandidate)
		}
	}
}

// AddRemoteCandidate adds a new remote candidate
func (a *Agent) AddRemoteCandidate(c Candidate) {
	a.Lock()
	defer a.Unlock()
	a.remoteCandidates = append(a.remoteCandidates, c)
}

// AddLocalCandidate adds a new local candidate
func (a *Agent) AddLocalCandidate(c Candidate) {
	a.Lock()
	defer a.Unlock()
	a.LocalCandidates = append(a.LocalCandidates, c)
}

// Close cleans up the Agent
func (a *Agent) Close() {
	if a.taskLoopChan != nil {
		close(a.taskLoopChan)
	}
}

func isCandidateMatch(c Candidate, testAddress string, testPort int) bool {
	if c.GetBase().Address == testAddress && c.GetBase().Port == testPort {
		return true
	}

	switch c := c.(type) {
	case *CandidateSrflx:
		if c.RemoteAddress == testAddress && c.RemotePort == testPort {
			return true
		}
	}

	return false
}

func getTransportAddrCandidate(candidates []Candidate, addr *stun.TransportAddr) Candidate {
	for _, c := range candidates {
		if isCandidateMatch(c, addr.IP.String(), addr.Port) {
			return c
		}
	}
	return nil
}

func getUDPAddrCandidate(candidates []Candidate, addr *net.UDPAddr) Candidate {
	for _, c := range candidates {
		if isCandidateMatch(c, addr.IP.String(), addr.Port) {
			return c
		}
	}
	return nil
}

func (a *Agent) sendBindingSuccess(m *stun.Message, local *stun.TransportAddr, remote *net.UDPAddr) {
	if out, err := stun.Build(stun.ClassSuccessResponse, stun.MethodBinding, m.TransactionID,
		&stun.XorMappedAddress{
			XorAddress: stun.XorAddress{
				IP:   remote.IP,
				Port: remote.Port,
			},
		},
		&stun.MessageIntegrity{
			Key: []byte(a.LocalPwd),
		},
		&stun.Fingerprint{},
	); err != nil {
		fmt.Printf("Failed to handle inbound ICE from: %s to: %s error: %s", local.String(), remote.String(), err.Error())
	} else {
		a.outboundCallback(out.Pack(), local, remote)
	}
}

func (a *Agent) handleInboundControlled(m *stun.Message, local *stun.TransportAddr, remote *net.UDPAddr, localCandidate, remoteCandidate Candidate) {
	if _, isControlled := m.GetOneAttribute(stun.AttrIceControlled); isControlled && !a.isControlling {
		fmt.Println("inbound isControlled && a.isControlling == false")
		return
	}

	successResponse := m.Method == stun.MethodBinding && m.Class == stun.ClassSuccessResponse
	_, usepair := m.GetOneAttribute(stun.AttrUseCandidate)
	// Remember the working pair and select it when marked with usepair
	a.setValidPair(localCandidate, remoteCandidate, usepair)

	if !successResponse {
		// Send success response
		a.sendBindingSuccess(m, local, remote)
	}
}

func (a *Agent) handleInboundControlling(m *stun.Message, local *stun.TransportAddr, remote *net.UDPAddr, localCandidate, remoteCandidate Candidate) {
	if _, isControlling := m.GetOneAttribute(stun.AttrIceControlling); isControlling && a.isControlling {
		fmt.Println("inbound isControlling && a.isControlling == true")
		return
	} else if _, useCandidate := m.GetOneAttribute(stun.AttrUseCandidate); useCandidate && a.isControlling {
		fmt.Println("useCandidate && a.isControlling == true")
		return
	}

	successResponse := m.Method == stun.MethodBinding && m.Class == stun.ClassSuccessResponse
	// Remember the working pair and select it when receiving a success response
	a.setValidPair(localCandidate, remoteCandidate, successResponse)

	if !successResponse {
		// Send success response
		a.sendBindingSuccess(m, local, remote)

		// We received a ping from the controlled agent. We know the pair works so now we ping with use-candidate set:
		a.pingCandidate(localCandidate, remoteCandidate)
	}
}

// HandleInbound processes traffic from a remote candidate
func (a *Agent) HandleInbound(buf []byte, local *stun.TransportAddr, remote *net.UDPAddr) {
	a.Lock()
	defer a.Unlock()

	localCandidate := getTransportAddrCandidate(a.LocalCandidates, local)
	if localCandidate == nil {
		// TODO debug
		// fmt.Printf("Could not find local candidate for %s:%d ", local.IP.String(), local.Port)
		return
	}

	remoteCandidate := getUDPAddrCandidate(a.remoteCandidates, remote)
	if remoteCandidate == nil {
		// TODO debug
		// fmt.Printf("Could not find remote candidate for %s:%d ", remote.IP.String(), remote.Port)
		return
	}

	remoteCandidate.GetBase().seen(false)

	m, err := stun.NewMessage(buf)
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to handle decode ICE from: %s to: %s error: %s", local.String(), remote.String(), err.Error()))
		return
	}

	if a.isControlling {
		a.handleInboundControlling(m, local, remote, localCandidate, remoteCandidate)
	} else {
		a.handleInboundControlled(m, local, remote, localCandidate, remoteCandidate)
	}

}

// SelectedPair gets the current selected pair's Addresses (or returns nil)
func (a *Agent) SelectedPair() (local *stun.TransportAddr, remote *net.UDPAddr) {
	a.RLock()
	defer a.RUnlock()

	if a.selectedPair.remote == nil || a.selectedPair.local == nil {
		for _, p := range a.validPairs {
			return p.getAddrs()
		}
		return nil, nil
	}

	return a.selectedPair.getAddrs()
}
