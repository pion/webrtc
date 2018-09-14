package ice

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAgent(t *testing.T) {
	agent1, err := NewAgent(nil)
	defer agent1.Close()
	assert.Nil(t, err)
	assert.True(t, len(agent1.transports) > 0)

	agent2, err := NewAgent(&[]*URL{{
		Scheme: SchemeTypeSTUN,
		Host:   "stun.l.google.com",
		Port:   19302,
		Proto:  ProtoTypeUDP,
	}})
	defer agent2.Close()
	assert.Nil(t, err)
	assert.True(t, len(agent2.transports) > len(agent1.transports))
}

func TestAgent_Start(t *testing.T) {
	agent1, err := NewAgent(nil)
	defer agent1.Close()
	assert.Nil(t, err)

	agent2, err := NewAgent(&[]*URL{{
		Scheme: SchemeTypeSTUN,
		Host:   "stun.l.google.com",
		Port:   19302,
		Proto:  ProtoTypeUDP,
	}})
	defer agent2.Close()
	assert.Nil(t, err)

	agent2.AddRemoteCandidate(agent1.LocalCandidates[0])
	agent2.Start(false, agent1.LocalUfrag, agent1.LocalPwd)

	agent1.AddRemoteCandidate(agent2.LocalCandidates[0])
	agent1.Start(true, agent2.LocalUfrag, agent2.LocalPwd)

	var wg sync.WaitGroup
	wg.Add(2)
	agent1.OnConnectionStateChange = func(state ConnectionState) {
		if state == ConnectionStateConnected {
			wg.Done()
		}
	}
	agent2.OnConnectionStateChange = func(state ConnectionState) {
		if state == ConnectionStateConnected {
			wg.Done()
		}
	}
	wg.Wait()

	local1, remote1 := agent1.SelectedPair()
	local2, remote2 := agent2.SelectedPair()
	assert.Equal(t, remote1.String(), local2.String())
	assert.Equal(t, remote2.String(), local1.String())
}
