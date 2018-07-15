package ice

import "github.com/pions/webrtc/internal/util"

// Agent represents the ICE agent
type Agent struct {
	Servers [][]URL
	Ufrag   string
	Pwd     string
}

// NewAgent creates a new Agent
func NewAgent() *Agent {
	return &Agent{
		Ufrag: util.RandSeq(16),
		Pwd:   util.RandSeq(32),
	}
}

// SetServers is used to set the ICE servers used by the Agent
func (a *Agent) SetServers(urls [][]URL) {
	a.Servers = urls
}
