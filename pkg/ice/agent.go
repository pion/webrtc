package ice

import (
	"fmt"
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
	state           ConnectionState
	gatheringState  GatheringState
	connectionState ConnectionState

	LocalUfrag      string
	LocalPwd        string
	localCandidates []Candidate

	remoteUfrag      string
	remotePwd        string
	remoteCandidates []Candidate
}

// NewAgent creates a new Agent
func NewAgent(isControlling bool, outboundCallback OutboundCallback) *Agent {
	return &Agent{
		isControlling:    isControlling,
		outboundCallback: outboundCallback,

		LocalUfrag: util.RandSeq(16),
		LocalPwd:   util.RandSeq(32),
	}
}

// AddLocalCandidate adds a new candidate
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

// HandleInbound processes traffic from a remote candidate
func (a *Agent) HandleInbound(buf []byte, local *stun.TransportAddr, remote *net.UDPAddr) {
	m, err := stun.NewMessage(buf)
	if err != nil {
		fmt.Printf("Failed to handle decode ICE from: %s to: %s error: %s", local.String(), remote.String(), err.Error())
	} else if m.Class != stun.ClassRequest {
		fmt.Printf("Wrong STUN Class ICE from: %s to: %s class: %s", local.String(), remote.String(), m.Class.String())
	} else if m.Method != stun.MethodBinding {
		fmt.Printf("Wrong STUN Method ICE from: %s to: %s method: %s", local.String(), remote.String(), m.Method.String())
	}

	msg, err := stun.Build(stun.ClassSuccessResponse, stun.MethodBinding, m.TransactionID,
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

	a.outboundCallback(msg.Pack(), local, remote)
}
