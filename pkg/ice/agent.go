package ice

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/util"
)

// OutboundCallback is the user defined Callback that is called when ICE traffic needs to sent
type OutboundCallback func(raw []byte, local *stun.TransportAddr, remote *net.UDPAddr)

// Agent represents the ICE agent
type Agent struct {
	sync.RWMutex

	outboundCallback OutboundCallback

	tieBreaker      uint64
	connectionState ConnectionState
	gatheringState  GatheringState

	haveStarted   bool
	isControlling bool
	taskLoopChan  chan bool

	LocalUfrag      string
	LocalPwd        string
	localCandidates []Candidate

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
	agentTickerBaseInterval = 20 * time.Millisecond
	stunTimeout             = 10 * time.Second
)

// NewAgent creates a new Agent
func NewAgent(outboundCallback OutboundCallback) *Agent {
	a := &Agent{
		tieBreaker:       rand.Uint64(),
		outboundCallback: outboundCallback,

		LocalUfrag: util.RandSeq(16),
		LocalPwd:   util.RandSeq(32),
	}
	if a.isControlling {
	}
	return a
}

// Start starts the agent
func (a *Agent) Start(isControlling bool, remoteUfrag, remotePwd string) {
	a.Lock()
	defer a.Unlock()

	if a.haveStarted {
		panic("Attempted to start agent twice")
	} else if remoteUfrag == "" {
		panic("remoteUfrag is empty")
	} else if remotePwd == "" {
		panic("remotePwd is empty")
	}

	a.isControlling = isControlling
	a.remoteUfrag = remoteUfrag
	a.remotePwd = remotePwd

	if isControlling {
		go a.agentControllingTaskLoop()
	}
}

func (a *Agent) pingCandidate(local, remote Candidate) {
	msg, err := stun.Build(stun.ClassRequest, stun.MethodBinding, stun.GenerateTransactionId(),
		&stun.Username{Username: a.remoteUfrag + ":" + a.LocalUfrag},
		&stun.UseCandidate{},
		&stun.IceControlling{TieBreaker: a.tieBreaker},
		&stun.MessageIntegrity{
			Key: []byte(a.LocalPwd),
		},
		&stun.Fingerprint{},
	)
	if err != nil {
		panic(err)
	}

	a.outboundCallback(msg.Pack(), &stun.TransportAddr{
		IP:   net.ParseIP(local.GetBase().Address),
		Port: local.GetBase().Port,
	}, &net.UDPAddr{
		IP:   net.ParseIP(remote.GetBase().Address),
		Port: remote.GetBase().Port,
	})
}

func (a *Agent) agentControllingTaskLoop() {
	// TODO this should be dynamic, and grow when the connection is stable
	t := time.NewTicker(agentTickerBaseInterval)

	for {
		select {
		case <-t.C:
			a.Lock()
			if a.selectedPair.remote == nil || a.selectedPair.local == nil {
				for _, localCandidate := range a.localCandidates {
					for _, remoteCandidate := range a.remoteCandidates {
						a.pingCandidate(localCandidate, remoteCandidate)
					}
				}
			} else {
				if time.Now().Sub(a.selectedPair.remote.GetBase().LastSeen) > stunTimeout {
					a.selectedPair.remote = nil
					a.selectedPair.local = nil
				} else {
					a.pingCandidate(a.selectedPair.local, a.selectedPair.remote)
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
	a.localCandidates = append(a.localCandidates, c)
}

// Close cleans up the Agent
func (a *Agent) Close() {
	close(a.taskLoopChan)
}

// LocalCandidates generates the string representation of the
// local candidates that can be used in the SDP
func (a *Agent) LocalCandidates() (rtrn []string) {
	a.Lock()
	defer a.Unlock()

	for _, c := range a.localCandidates {
		rtrn = append(rtrn, c.String(1))
		rtrn = append(rtrn, c.String(2))
	}
	return rtrn
}

func getTransportAddrCandidate(candidates []Candidate, addr *stun.TransportAddr) Candidate {
	for _, c := range candidates {
		if c.GetBase().Address == addr.IP.String() && c.GetBase().Port == addr.Port {
			return c
		}
	}
	return nil
}

func getUDPAddrCandidate(candidates []Candidate, addr *net.UDPAddr) Candidate {
	for _, c := range candidates {
		if c.GetBase().Address == addr.IP.String() && c.GetBase().Port == addr.Port {
			return c
		}
	}
	return nil
}

func (a *Agent) handleInboundControlled(m *stun.Message, local *stun.TransportAddr, remote *net.UDPAddr, localCandidate, remoteCandidate Candidate) {
	if _, isControlled := m.GetOneAttribute(stun.AttrIceControlled); isControlled && a.isControlling == false {
		panic("inbound isControlled && a.isControlling == false")
	} else if m.Class != stun.ClassRequest {
		panic(fmt.Sprintf("Wrong STUN Class ICE from: %s to: %s class: %s", local.String(), remote.String(), m.Class.String()))
	} else if m.Method != stun.MethodBinding {
		panic(fmt.Sprintf("Wrong STUN Method ICE from: %s to: %s method: %s", local.String(), remote.String(), m.Method.String()))
	}

	if _, useCandidateFound := m.GetOneAttribute(stun.AttrUseCandidate); useCandidateFound {
		a.selectedPair.remote = remoteCandidate
		a.selectedPair.local = localCandidate
	}
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

func (a *Agent) handleInboundControlling(m *stun.Message, local *stun.TransportAddr, remote *net.UDPAddr, localCandidate, remoteCandidate Candidate) {
	if _, isControlling := m.GetOneAttribute(stun.AttrIceControlling); isControlling && a.isControlling == true {
		panic("inbound isControlling && a.isControlling == true")
	} else if _, useCandidate := m.GetOneAttribute(stun.AttrUseCandidate); useCandidate && a.isControlling == true {
		panic("useCandidate && a.isControlling == true")
	}

	if m.Class != stun.ClassSuccessResponse || m.Method != stun.MethodBinding {
		if out, err := stun.Build(stun.ClassErrorResponse, stun.MethodBinding, m.TransactionID,
			&stun.Err401Unauthorized,
			&stun.MessageIntegrity{
				Key: []byte(a.LocalPwd),
			},
			&stun.Fingerprint{},
		); err != nil {
			fmt.Printf("Failed to handle inbound ICE from: %s to: %s error: %s", local.String(), remote.String(), err.Error())
		} else {
			a.outboundCallback(out.Pack(), local, remote)
		}
	} else if a.selectedPair.remote == nil && a.selectedPair.local == nil {
		//Binding success!
		a.selectedPair.remote = remoteCandidate
		a.selectedPair.local = localCandidate
	}
}

// HandleInbound processes traffic from a remote candidate
func (a *Agent) HandleInbound(buf []byte, local *stun.TransportAddr, remote *net.UDPAddr) {
	a.Lock()
	defer a.Unlock()

	localCandidate := getTransportAddrCandidate(a.localCandidates, local)
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
	remoteCandidate.GetBase().LastSeen = time.Now()

	m, err := stun.NewMessage(buf)
	if err != nil {
		panic(fmt.Sprintf("Failed to handle decode ICE from: %s to: %s error: %s", local.String(), remote.String(), err.Error()))
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
