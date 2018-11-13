package ice

import (
	"fmt"

	"github.com/pions/pkg/stun"
)

func newCandidatePair(local, remote Candidate) *candidatePair {
	return &candidatePair{
		remote: remote,
		local:  local,
	}
}

// candidatePair represents a combination of a local and remote candidate
type candidatePair struct {
	remote Candidate
	local  Candidate
}

func (p *candidatePair) Write(b []byte) (int, error) {
	return p.local.GetBase().writeTo(b, p.remote.GetBase())
}

// keepaliveCandidate sends a STUN Binding Indication to the remote candidate
func (a *Agent) keepaliveCandidate(local, remote Candidate) {
	msg, err := stun.Build(stun.ClassIndication, stun.MethodBinding, stun.GenerateTransactionId(),
		&stun.Username{Username: a.remoteUfrag + ":" + a.localUfrag},
		&stun.MessageIntegrity{
			Key: []byte(a.remotePwd),
		},
		&stun.Fingerprint{},
	)

	if err != nil {
		fmt.Println(err)
		return
	}

	a.sendSTUN(msg, local, remote)
}

func (a *Agent) sendSTUN(msg *stun.Message, local, remote Candidate) {
	_, err := local.GetBase().writeTo(msg.Pack(), remote.GetBase())
	if err != nil {
		// TODO: Determine if we should always drop the err
		// E.g.: maybe handle for known valid pairs or to
		// discard pairs faster.
		_ = err
		// fmt.Printf("failed to send STUN message: %v", err)
	}
}
