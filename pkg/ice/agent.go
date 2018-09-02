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

// GetAddrs returns network addresses for the candidate pair
func (c CandidatePair) GetAddrs() (local *stun.TransportAddr, remote *net.UDPAddr) {
	return &stun.TransportAddr{
			IP:   net.ParseIP(c.local.GetBase().Address),
			Port: c.local.GetBase().Port,
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
	agentTickerBaseInterval = 3 * time.Second
	stunTimeout             = 10 * time.Second
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

	go a.agentTaskLoop()
	return nil
}

func (a *Agent) pingCandidate(local, remote Candidate) {
	if a.isControlling {
		msg, err := stun.Build(stun.ClassRequest, stun.MethodBinding, stun.GenerateTransactionId(),
			&stun.Username{Username: a.remoteUfrag + ":" + a.LocalUfrag},
			&stun.UseCandidate{},
			&stun.IceControlling{TieBreaker: a.tieBreaker},
			&stun.Priority{Priority: uint32(local.GetBase().Priority(HostCandidatePreference, 1))},
			&stun.MessageIntegrity{
				Key: []byte(a.remotePwd),
			},
			&stun.Fingerprint{},
		)
		if err != nil {
			fmt.Println(err)
			return
		}

		a.outboundCallback(msg.Pack(), &stun.TransportAddr{
			IP:   net.ParseIP(local.GetBase().Address),
			Port: local.GetBase().Port,
		}, &net.UDPAddr{
			IP:   net.ParseIP(remote.GetBase().Address),
			Port: remote.GetBase().Port,
		})
	} else {
		msg, err := stun.Build(stun.ClassRequest, stun.MethodBinding, stun.GenerateTransactionId(),
			&stun.Username{Username: a.remoteUfrag + ":" + a.LocalUfrag},
			&stun.UseCandidate{},
			&stun.IceControlled{TieBreaker: a.tieBreaker},
			&stun.Priority{Priority: uint32(local.GetBase().Priority(HostCandidatePreference, 1))},
			&stun.MessageIntegrity{
				Key: []byte(a.remotePwd),
			},
			&stun.Fingerprint{},
		)
		if err != nil {
			fmt.Println(err)
			return
		}

		a.outboundCallback(msg.Pack(), &stun.TransportAddr{
			IP:   net.ParseIP(local.GetBase().Address),
			Port: local.GetBase().Port,
		}, &net.UDPAddr{
			IP:   net.ParseIP(remote.GetBase().Address),
			Port: remote.GetBase().Port,
		})
	}
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

func (a *Agent) agentTaskLoop() {
	// TODO this should be dynamic, and grow when the connection is stable
	t := time.NewTicker(agentTickerBaseInterval)
	a.updateConnectionState(ConnectionStateChecking)

	assertSelectedPairValid := func() bool {
		if a.selectedPair.remote == nil || a.selectedPair.local == nil {
			return false
		} else if time.Since(a.selectedPair.remote.GetBase().LastSeen) > stunTimeout {
			a.selectedPair.remote = nil
			a.selectedPair.local = nil
			a.updateConnectionState(ConnectionStateDisconnected)
			return false
		}

		return true
	}

	for {
		select {
		case <-t.C:
			a.Lock()
			if a.isControlling {
				if assertSelectedPairValid() {
					a.Unlock()
					continue
				}

				for _, localCandidate := range a.LocalCandidates {
					for _, remoteCandidate := range a.remoteCandidates {
						a.pingCandidate(localCandidate, remoteCandidate)
					}
				}
			} else {
				if assertSelectedPairValid() {
					a.Unlock()
					continue
				}

				for _, localCandidate := range a.LocalCandidates {
					for _, remoteCandidate := range a.remoteCandidates {
						a.pingCandidate(localCandidate, remoteCandidate)
					}
				}
			}
			a.Unlock()
		case <-a.taskLoopChan:
			t.Stop()
			return
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

	_, useCandidateFound := m.GetOneAttribute(stun.AttrUseCandidate)
	a.setValidPair(localCandidate, remoteCandidate, useCandidateFound)

	a.sendBindingSuccess(m, local, remote)
}

func (a *Agent) handleInboundControlling(m *stun.Message, local *stun.TransportAddr, remote *net.UDPAddr, localCandidate, remoteCandidate Candidate) {
	if _, isControlling := m.GetOneAttribute(stun.AttrIceControlling); isControlling && a.isControlling {
		fmt.Println("inbound isControlling && a.isControlling == true")
		return
	} else if _, useCandidate := m.GetOneAttribute(stun.AttrUseCandidate); useCandidate && a.isControlling {
		fmt.Println("useCandidate && a.isControlling == true")
		return
	}

	final := m.Class == stun.ClassSuccessResponse && m.Method == stun.MethodBinding
	a.setValidPair(localCandidate, remoteCandidate, final)

	if !final {
		a.sendBindingSuccess(m, local, remote)
	}
}

// HandleInbound processes traffic from a remote candidate
func (a *Agent) HandleInbound(buf []byte, local *stun.TransportAddr, remote *net.UDPAddr) {
	a.Lock()
	defer a.Unlock()

	localCandidate := getTransportAddrCandidate(a.LocalCandidates, local)
	if localCandidate == nil {
		// TODO debug
		// fmt.Printf("Could not find local candidate for %s:%d ", local.IP.String(), local.Value)
		return
	}

	remoteCandidate := getUDPAddrCandidate(a.remoteCandidates, remote)
	if remoteCandidate == nil {
		// TODO debug
		// fmt.Printf("Could not find remote candidate for %s:%d ", remote.IP.String(), remote.Value)
		return
	}
	remoteCandidate.GetBase().LastSeen = time.Now()

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
			return p.GetAddrs()
		}
		return nil, nil
	}

	return a.selectedPair.GetAddrs()
}
