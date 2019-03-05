package ice

import (
	"net"
	"testing"
	"time"

	"github.com/pions/transport/test"
)

func TestPairSearch(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	var config AgentConfig
	a, err := NewAgent(&config)

	if err != nil {
		t.Fatalf("Error constructing ice.Agent")
	}

	if len(a.validPairs) != 0 {
		t.Fatalf("TestPairSearch is only a valid test if a.validPairs is empty on construction")
	}

	cp, err := a.getBestPair()

	if cp != nil {
		t.Fatalf("No Candidate pairs should exist")
	}

	if err == nil {
		t.Fatalf("An error should have been reported (with no available candidate pairs)")
	}

	err = a.Close()

	if err != nil {
		t.Fatalf("Close agent emits error %v", err)
	}
}

func TestPairPriority(t *testing.T) {
	// avoid deadlocks?
	defer test.TimeOut(1 * time.Second).Stop()

	a, err := NewAgent(&AgentConfig{})
	if err != nil {
		t.Fatalf("Failed to create agent: %s", err)
	}

	hostLocal, err := NewCandidateHost(
		"udp",
		net.ParseIP("192.168.1.1"), 19216,
		1,
	)
	if err != nil {
		t.Fatalf("Failed to construct local host candidate: %s", err)
	}

	relayRemote, err := NewCandidateRelay(
		"udp",
		net.ParseIP("1.2.3.4"), 12340,
		1,
		"4.3.2.1", 43210,
	)
	if err != nil {
		t.Fatalf("Failed to construct remote relay candidate: %s", err)
	}

	srflxRemote, err := NewCandidateServerReflexive(
		"udp",
		net.ParseIP("10.10.10.2"), 19218,
		1,
		"4.3.2.1", 43212,
	)
	if err != nil {
		t.Fatalf("Failed to construct remote srflx candidate: %s", err)
	}

	prflxRemote, err := NewCandidatePeerReflexive(
		"udp",
		net.ParseIP("10.10.10.2"), 19217,
		1,
		"4.3.2.1", 43211,
	)
	if err != nil {
		t.Fatalf("Failed to construct remote prflx candidate: %s", err)
	}

	hostRemote, err := NewCandidateHost(
		"udp",
		net.ParseIP("1.2.3.5"), 12350,
		1,
	)
	if err != nil {
		t.Fatalf("Failed to construct remote host candidate: %s", err)
	}

	for _, remote := range []*Candidate{relayRemote, srflxRemote, prflxRemote, hostRemote} {
		a.setValidPair(hostLocal, remote, false, false)
		bestPair, err := a.getBestPair()
		if err != nil {
			t.Fatalf("Failed to get best candidate pair: %s", err)
		}
		if bestPair.String() != (&candidatePair{remote: remote, local: hostLocal}).String() {
			t.Fatalf("Unexpected bestPair %s (expected remote: %s)", bestPair, remote)
		}
	}

	if err := a.Close(); err != nil {
		t.Fatalf("Error on agent.Close(): %s", err)
	}
}

func TestOnSelectedCandidatePairChange(t *testing.T) {
	// avoid deadlocks?
	defer test.TimeOut(1 * time.Second).Stop()

	a, err := NewAgent(&AgentConfig{})
	if err != nil {
		t.Fatalf("Failed to create agent: %s", err)
	}
	callbackCalled := make(chan struct{}, 1)
	if err = a.OnSelectedCandidatePairChange(func(local, remote *Candidate) {
		close(callbackCalled)
	}); err != nil {
		t.Fatalf("Failed to set agent OnCandidatePairChange callback: %s", err)
	}

	hostLocal, err := NewCandidateHost(
		"udp",
		net.ParseIP("192.168.1.1"), 19216,
		1,
	)
	if err != nil {
		t.Fatalf("Failed to construct local host candidate: %s", err)
	}

	relayRemote, err := NewCandidateRelay(
		"udp",
		net.ParseIP("1.2.3.4"), 12340,
		1,
		"4.3.2.1", 43210,
	)
	if err != nil {
		t.Fatalf("Failed to construct remote relay candidate: %s", err)
	}

	// select the pair
	if err = a.run(func(agent *Agent) {
		agent.setValidPair(hostLocal, relayRemote, true, false)
	}); err != nil {
		t.Fatalf("Failed to setValidPair(): %s", err)
	}
	// ensure that the callback fired on setting the pair
	<-callbackCalled
	// set the same pair; this should not invoke the callback
	// if the callback is invoked now it will panic due
	// to second close of the channel
	if err = a.run(func(agent *Agent) {
		agent.setValidPair(hostLocal, relayRemote, true, false)
	}); err != nil {
		t.Fatalf("Failed to setValidPair(): %s", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Error on agent.Close(): %s", err)
	}
}

type BadAddr struct{}

func (ba *BadAddr) Network() string {
	return "xxx"
}
func (ba *BadAddr) String() string {
	return "yyy"
}

func TestHandlePeerReflexive(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 2)
	defer lim.Stop()

	t.Run("UDP pflx candidate from handleInboud()", func(t *testing.T) {
		var config AgentConfig
		a, err := NewAgent(&config)

		if err != nil {
			t.Fatalf("Error constructing ice.Agent")
		}

		ip := net.ParseIP("192.168.0.2")
		local, err := NewCandidateHost("udp", ip, 777, 1)
		if err != nil {
			t.Fatalf("failed to create a new candidate: %v", err)
		}

		remote := &net.UDPAddr{IP: net.ParseIP("172.17.0.3"), Port: 999}

		a.handleInbound(nil, local, remote)

		// length of remote candidate list must be one now
		if len(a.remoteCandidates) != 1 {
			t.Fatal("failed to add a network type to the remote candidate list")
		}

		// length of remote candidate list for a network type must be 1
		set := a.remoteCandidates[local.NetworkType]
		if len(set) != 1 {
			t.Fatal("failed to add prflx candidate to remote candidate list")
		}

		c := set[0]

		if c.Type != CandidateTypePeerReflexive {
			t.Fatal("candidate type must be prflx")
		}

		if !c.IP.Equal(net.ParseIP("172.17.0.3")) {
			t.Fatal("IP address mismatch")
		}

		if c.Port != 999 {
			t.Fatal("Port number mismatch")
		}

		err = a.Close()
		if err != nil {
			t.Fatalf("Close agent emits error %v", err)
		}
	})

	t.Run("Bad network type with handleInbound()", func(t *testing.T) {
		var config AgentConfig
		a, err := NewAgent(&config)

		if err != nil {
			t.Fatal("Error constructing ice.Agent")
		}

		ip := net.ParseIP("192.168.0.2")
		local, err := NewCandidateHost("tcp", ip, 777, 1)
		if err != nil {
			t.Fatalf("failed to create a new candidate: %v", err)
		}

		remote := &BadAddr{}

		a.handleInbound(nil, local, remote)

		if len(a.remoteCandidates) != 0 {
			t.Fatal("bad address should not be added to the remote candidate list")
		}

		err = a.Close()
		if err != nil {
			t.Fatalf("Close agent emits error %v", err)
		}
	})

	t.Run("TCP prflx with handleNewPeerReflexiveCandidate()", func(t *testing.T) {
		var config AgentConfig
		a, err := NewAgent(&config)

		if err != nil {
			t.Fatal("Error constructing ice.Agent")
		}

		ip := net.ParseIP("192.168.0.2")
		local, err := NewCandidateHost("tcp", ip, 777, 1)
		if err != nil {
			t.Fatalf("failed to create a new candidate: %v", err)
		}

		remote := &net.TCPAddr{IP: net.ParseIP("172.17.0.3"), Port: 999}

		err = a.handleNewPeerReflexiveCandidate(local, remote)
		if err != nil {
			t.Fatalf("handleNewPeerReflexiveCandidate() should not fail: %v", err)
		}

		// length of remote candidate list must be one now
		if len(a.remoteCandidates) != 1 {
			t.Fatal("failed to add a network type to the remote candidate list")
		}

		// length of remote candidate list for a network type must be 1
		set := a.remoteCandidates[local.NetworkType]
		if len(set) != 1 {
			t.Fatal("failed to add prflx candidate to remote candidate list")
		}

		c := set[0]

		if c.Type != CandidateTypePeerReflexive {
			t.Fatal("candidate type must be prflx")
		}

		if !c.IP.Equal(net.ParseIP("172.17.0.3")) {
			t.Fatal("IP address mismatch")
		}

		if c.Port != 999 {
			t.Fatal("Port number mismatch")
		}

		err = a.Close()
		if err != nil {
			t.Fatalf("Close agent emits error %v", err)
		}
	})
}
