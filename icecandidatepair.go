package webrtc

import (
	"fmt"

	"github.com/pion/ice/v2"
)
// ICECandidatePair represents an ICE Candidate pair
type ICECandidatePair struct {
	statsID string
	Local   *ice.ICECandidate
	Remote  *ice.ICECandidate
}

func newICECandidatePairStatsID(localID, remoteID string) string {
	return fmt.Sprintf("%s-%s", localID, remoteID)
}

func (p *ICECandidatePair) String() string {
	return fmt.Sprintf("(local) %s <-> (remote) %s", p.Local, p.Remote)
}

// NewICECandidatePair returns an initialized *ICECandidatePair
// for the given pair of ICECandidate instances
func NewICECandidatePair(local, remote *ice.ICECandidate) *ICECandidatePair {
	statsID := newICECandidatePairStatsID(local.StatsID, remote.StatsID)
	return &ICECandidatePair{
		statsID: statsID,
		Local:   local,
		Remote:  remote,
	}
}
