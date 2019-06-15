package webrtc

import "fmt"

type (
	// ICECandidatePair represents an ICE Candidate pair
	ICECandidatePair struct {
		Local  *ICECandidate
		Remote *ICECandidate
	}
)

func (p *ICECandidatePair) String() string {
	return fmt.Sprintf("(local) %s <-> (remote) %s", p.Local, p.Remote)
}

// NewICECandidatePair returns an initialized *ICECandidatePair
// for the given pair of ICECandidate instances
func NewICECandidatePair(local, remote *ICECandidate) *ICECandidatePair {
	return &ICECandidatePair{
		Local:  local,
		Remote: remote,
	}
}
