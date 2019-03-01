package ice

import (
	"fmt"

	"github.com/pions/stun"
)

func newCandidatePair(local, remote *Candidate, controlling bool) *candidatePair {
	return &candidatePair{
		iceRoleControlling: controlling,
		remote:             remote,
		local:              local,
	}
}

// candidatePair represents a combination of a local and remote candidate
type candidatePair struct {
	iceRoleControlling bool
	remote             *Candidate
	local              *Candidate
}

func (p *candidatePair) String() string {
	return fmt.Sprintf("prio %d (local, prio %d) %s <-> %s (remote, prio %d)",
		p.Priority(), p.local.Priority(), p.local, p.remote, p.remote.Priority())
}

func (p *candidatePair) Equal(other *candidatePair) bool {
	if p == nil && other == nil {
		return true
	}
	if p == nil || other == nil {
		return false
	}
	return p.local.Equal(other.local) && p.remote.Equal(other.remote)
}

// RFC 5245 - 5.7.2.  Computing Pair Priority and Ordering Pairs
// Let G be the priority for the candidate provided by the controlling
// agent.  Let D be the priority for the candidate provided by the
// controlled agent.
// pair priority = 2^32*MIN(G,D) + 2*MAX(G,D) + (G>D?1:0)
func (p *candidatePair) Priority() uint32 {
	var g uint32
	var d uint32
	if p.iceRoleControlling {
		g = uint32(p.local.Priority())
		d = uint32(p.remote.Priority())
	} else {
		g = uint32(p.remote.Priority())
		d = uint32(p.local.Priority())
	}

	// Just implement these here rather
	// than fooling around with the math package
	min := func(x, y uint32) uint32 {
		if x < y {
			return x
		}
		return y
	}
	max := func(x, y uint32) uint32 {
		if x > y {
			return x
		}
		return y
	}
	cmp := func(x, y uint32) uint32 {
		if x > y {
			return 1
		}
		return 0
	}

	return (2^32)*min(g, d) + 2*max(g, d) + cmp(g, d)
}

func (p *candidatePair) Write(b []byte) (int, error) {
	return p.local.writeTo(b, p.remote)
}

// keepaliveCandidate sends a STUN Binding Indication to the remote candidate
func (a *Agent) keepaliveCandidate(local, remote *Candidate) {
	msg, err := stun.Build(stun.ClassIndication, stun.MethodBinding, stun.GenerateTransactionID(),
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
		iceLog.Tracef("failed to send STUN message: %s", err)
	}
}
