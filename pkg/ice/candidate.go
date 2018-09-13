package ice

import (
	"math/rand"
	"time"
)

// Preference enums when generate Priority
const (
	HostCandidatePreference  uint16 = 126
	SrflxCandidatePreference uint16 = 100
)

// Candidate represents an ICE candidate
type Candidate interface {
	Base() *baseCandidate
}

// CandidateBase represents an ICE candidate, a base with enough attributes
// for host candidates, see CandidateSrflx and CandidateRelay for more
type baseCandidate struct {
	Protocol ProtoType
	Address  string
	Port     int
	LastSeen time.Time
}

// Priority computes the priority for this ICE Candidate
func (c *baseCandidate) Priority(typePreference uint16, component uint16) uint16 {
	localPreference := uint16(rand.New(rand.NewSource(time.Now().UnixNano())).Uint32() / 2)
	return (2^24)*typePreference +
		(2^8)*localPreference +
		(2^0)*(256-component)
}

// HostCandidate is a Candidate of typ Host
type HostCandidate struct {
	baseCandidate
}

// GetBase returns the CandidateBase, attributes shared between all Candidates
func (c *HostCandidate) Base() *baseCandidate {
	return &c.baseCandidate
}

// Address for CandidateHost
func (c *HostCandidate) Address() string {
	return c.baseCandidate.Address
}

// Port for CandidateHost
func (c *HostCandidate) Port() int {
	return c.baseCandidate.Port
}

// CandidateSrflx is a Candidate of typ Server-Reflexive
type SrflxCandidate struct {
	baseCandidate
	RemoteAddress string
	RemotePort    int
}

// GetBase returns the CandidateBase, attributes shared between all Candidates
func (c *SrflxCandidate) Base() *baseCandidate {
	return &c.baseCandidate
}
