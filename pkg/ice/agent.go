package ice

import (
	"fmt"
	"math/rand"
	"net"
	"sync"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/util"
)

// OutboundCallback is the user defined Callback that is called when ICE traffic needs to sent
type OutboundCallback func(raw []byte, local *stun.TransportAddr, remote *net.UDPAddr)

// Agent represents the ICE agent
type Agent struct {
	sync.RWMutex

	outboundCallback OutboundCallback

	isControlling   bool
	tieBreaker      uint32
	state           ConnectionState
	gatheringState  GatheringState
	connectionState ConnectionState

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

// NewAgent creates a new Agent
func NewAgent(isControlling bool, outboundCallback OutboundCallback) *Agent {
	return &Agent{
		isControlling:    isControlling,
		tieBreaker:       rand.Uint32(),
		outboundCallback: outboundCallback,

		LocalUfrag: util.RandSeq(16),
		LocalPwd:   util.RandSeq(32),
	}
}

func (a *Agent) agentInterval() {
	// TODO
	// If isControlling we need to send out
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
		if c.Address() == addr.IP.String() && c.Port() == addr.Port {
			return c
		}
	}
	return nil
}

func getUDPAddrCandidate(candidates []Candidate, addr *net.UDPAddr) Candidate {
	for _, c := range candidates {
		if c.Address() == addr.IP.String() && c.Port() == addr.Port {
			return c
		}
	}

	return nil
}

// HandleInbound processes traffic from a remote candidate
func (a *Agent) HandleInbound(buf []byte, local *stun.TransportAddr, remote *net.UDPAddr) {
	a.Lock()
	defer a.Unlock()

	localCandidate := getTransportAddrCandidate(a.localCandidates, local)
	if localCandidate == nil {
		fmt.Printf("Could not find local candidate for %s:%d ", local.IP.String(), local.Port)
		return
	}

	remoteCandidate := getUDPAddrCandidate(a.remoteCandidates, remote)
	if remoteCandidate == nil {
		fmt.Printf("Could not find remote candidate for %s:%d ", remote.IP.String(), remote.Port)
		return
	}

	m, err := stun.NewMessage(buf)
	if err != nil {
		fmt.Printf("Failed to handle decode ICE from: %s to: %s error: %s", local.String(), remote.String(), err.Error())
	} else if m.Class != stun.ClassRequest {
		fmt.Printf("Wrong STUN Class ICE from: %s to: %s class: %s", local.String(), remote.String(), m.Class.String())
	} else if m.Method != stun.MethodBinding {
		fmt.Printf("Wrong STUN Method ICE from: %s to: %s method: %s", local.String(), remote.String(), m.Method.String())
	}

	if _, useCandidateFound := m.GetOneAttribute(stun.AttrUseCandidate); useCandidateFound {
		a.selectedPair.remote = remoteCandidate
		a.selectedPair.local = localCandidate
	}

	// iceControlledAttr := &stun.IceControlled{}
	// realmRawAttr, realmFound := m.GetOneAttribute(stun.AttrRealm);

	// iceControllingAttr := &stun.IceControlling{}
	// priorityAttr := &stun.Priority{}

	// Handle, maybe update properties
	out, err := stun.Build(stun.ClassSuccessResponse, stun.MethodBinding, m.TransactionID,
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
	)
	if err != nil {
		fmt.Printf("Failed to handle inbound ICE from: %s to: %s error: %s", local.String(), remote.String(), err.Error())
	}

	a.outboundCallback(out.Pack(), local, remote)
}

// SelectedPair gets the current selected pair's Addresses (or returns nil)
func (a *Agent) SelectedPair() (local *stun.TransportAddr, remote *net.UDPAddr) {
	a.RLock()
	defer a.RUnlock()

	if a.selectedPair.remote == nil || a.selectedPair.local == nil {
		return nil, nil
	}

	return &stun.TransportAddr{
			IP:   net.ParseIP(a.selectedPair.local.Address()),
			Port: a.selectedPair.local.Port(),
		}, &net.UDPAddr{
			IP:   net.ParseIP(a.selectedPair.remote.Address()),
			Port: a.selectedPair.remote.Port(),
		}
}
