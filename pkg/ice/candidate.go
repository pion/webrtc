package ice

// Candidate represents an ICE candidate
type Candidate interface {
	String(component int) string
}

// CandidateBase represents an ICE candidate, a base with enough attributes
// for host candidates, see CandidateSrflx and CandidateRelay for more
type CandidateBase struct {
	Protocol TransportType
	Address  string
	Port     int
}

// CandidateHost is a Candidate of typ Host
type CandidateHost struct {
	CandidateBase
}

// String for CandidateHost
func (c *CandidateHost) String(component int) string {
	return ""
}

// CandidateSrflx is a Candidate of typ Server-Reflexive
type CandidateSrflx struct {
	CandidateBase
	RemoteAddress string
	RemotePort    int
}

// String for CandidateSrflx
func (c *CandidateSrflx) String(component int) string {
	return ""
}
