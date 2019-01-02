package ice

import (
	"github.com/pions/pkg/stun"
)

func newCandidatePair(local, remote *Candidate) *candidatePair {
	return &candidatePair{
		remote: remote,
		local:  local,
	}
}

// candidatePair represents a combination of a local and remote candidate
type candidatePair struct {
	remote *Candidate
	local  *Candidate
}

func (p *candidatePair) Write(b []byte) (int, error) {
	return p.local.writeTo(b, p.remote)
}

// keepaliveCandidate sends a STUN Binding Indication to the remote candidate
func (a *Agent) keepaliveCandidate(local, remote *Candidate) {
	msg, err := stun.Build(stun.ClassIndication, stun.MethodBinding, stun.GenerateTransactionId(),
		&stun.Username{Username: a.remoteUfrag + ":" + a.localUfrag},
		&stun.MessageIntegrity{
			Key: []byte(a.remotePwd),
		},
		&stun.Fingerprint{},
	)

	if err != nil {
		iceLog.Warn(err.Error())
		return
	}

	a.sendSTUN(msg, local, remote)
}

func (a *Agent) sendSTUN(msg *stun.Message, local, remote *Candidate) {
	_, err := local.writeTo(msg.Pack(), remote)
	if err != nil {
		iceLog.Warnf("failed to send STUN message: %s", err)
	}
}
