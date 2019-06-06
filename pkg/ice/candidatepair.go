package ice

import "fmt"

type (
	// CandidatePair represents an ICE Candidate pair
	CandidatePair struct {
		Local  *Candidate
		Remote *Candidate
	}
)

func (p *CandidatePair) String() string {
	return fmt.Sprintf("(local) %s <-> (remote) %s", p.Local, p.Remote)
}

// NewCandidatePair returns an initialized *CandidatePair
// for the given pair of Candidate instances
func NewCandidatePair(local, remote *Candidate) *CandidatePair {
	return &CandidatePair{
		Local:  local,
		Remote: remote,
	}
}
