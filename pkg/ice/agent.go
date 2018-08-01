package ice

import (
	"fmt"
	"net"
	"sync"

	"github.com/pions/webrtc/internal/util"
)

// Agent represents the ICE agent
type Agent struct {
	sync.RWMutex

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
func NewAgent(isControlling bool) *Agent {
	return &Agent{
		isControlling: isControlling,

		LocalUfrag: util.RandSeq(16),
		LocalPwd:   util.RandSeq(32),
	}
}

// AddLocalCandidate adds a new candidate
func (a *Agent) AddLocalCandidate(c Candidate) {
}

// Close cleans up the Agent
func (a *Agent) Close() {
}

// LocalCandidates generates the string representation of the
// local candidates that can be used in the SDP
func (a *Agent) LocalCandidates() []string {
	return nil
}

// HandleInbound processes traffic from a remote candidate
func (a *Agent) HandleInbound(buf []byte, addr *net.UDPAddr) {
	fmt.Println("ICE Traffic!")
}
