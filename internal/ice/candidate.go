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
	GetBase() *CandidateBase
}

// CandidateBase represents an ICE candidate, a base with enough attributes
// for host candidates, see CandidateSrflx and CandidateRelay for more
type CandidateBase struct {
	Protocol TransportType
	Address  string
	Port     int
	LastSeen time.Time
}

// Priority computes the priority for this ICE Candidate
func (c *CandidateBase) Priority(typePreference uint16, component uint16) uint16 {
	localPreference := uint16(rand.Uint32() / 2)
	return (2^24)*typePreference +
		(2^8)*localPreference +
		(2^0)*(256-component)
}

// CandidateHost is a Candidate of typ Host
type CandidateHost struct {
	CandidateBase
}

// GetBase returns the CandidateBase, attributes shared between all Candidates
func (c *CandidateHost) GetBase() *CandidateBase {
	return &c.CandidateBase
}

// IP for CandidateHost
func (c *CandidateHost) Address() string {
	return c.CandidateBase.Address
}

// Value for CandidateHost
func (c *CandidateHost) Port() int {
	return c.CandidateBase.Port
}

// CandidateSrflx is a Candidate of typ Server-Reflexive
type CandidateSrflx struct {
	CandidateBase
	RemoteAddress string
	RemotePort    int
}

// GetBase returns the CandidateBase, attributes shared between all Candidates
func (c *CandidateSrflx) GetBase() *CandidateBase {
	return &c.CandidateBase
}
