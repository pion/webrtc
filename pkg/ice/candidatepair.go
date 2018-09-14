package ice

import (
	"net"
)

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
func (c CandidatePair) GetAddrs() (local *net.UDPAddr, remote *net.UDPAddr) {
	localAddr := net.UDPAddr{}
	localAddr.IP, localAddr.Zone = splitIPZone(c.local.GetBase().Address)
	localAddr.Port = c.local.GetBase().Port

	remoteAddr := net.UDPAddr{}
	remoteAddr.IP, remoteAddr.Zone = splitIPZone(c.remote.GetBase().Address)
	remoteAddr.Port = c.remote.GetBase().Port

	return &localAddr, &remoteAddr
}
