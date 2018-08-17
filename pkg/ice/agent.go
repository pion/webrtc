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

// OutboundCallback is the user defined Callback that is called when ICE traffic needs to sent
type OutboundCallback func(raw []byte, local *stun.TransportAddr, remote *net.UDPAddr)

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

	selectedPair struct {
		// lastUpdateTime ?
		remote Candidate
		local  Candidate
	}
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

		tieBreaker:      rand.Uint64(),
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
}

func (a *Agent) updateConnectionState(newState ConnectionState) {
	a.connectionState = newState
	a.iceNotifier(a.connectionState)
}

func (a *Agent) setSelectedPair(local, remote Candidate) {
	a.selectedPair.remote = remote
	a.selectedPair.local = local
	a.updateConnectionState(ConnectionStateConnected)
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
				assertSelectedPairValid()
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
	close(a.taskLoopChan)
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

	if _, useCandidateFound := m.GetOneAttribute(stun.AttrUseCandidate); useCandidateFound {
		a.setSelectedPair(localCandidate, remoteCandidate)
	}
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

	if m.Class == stun.ClassSuccessResponse && m.Method == stun.MethodBinding {
		//Binding success!
		if a.selectedPair.remote == nil && a.selectedPair.local == nil {
			a.setSelectedPair(localCandidate, remoteCandidate)
		}
	} else {
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
		return nil, nil
	}

	return &stun.TransportAddr{
			IP:   net.ParseIP(a.selectedPair.local.GetBase().Address),
			Port: a.selectedPair.local.GetBase().Port,
		}, &net.UDPAddr{
			IP:   net.ParseIP(a.selectedPair.remote.GetBase().Address),
			Port: a.selectedPair.remote.GetBase().Port,
		}
}
