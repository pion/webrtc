// +build !js

package webrtc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pion/transport/test"
	"github.com/stretchr/testify/assert"
)

func TestNewICEGatherer_Success(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	opts := ICEGatherOptions{
		ICEServers: []ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}

	gatherer, err := NewAPI().NewICEGatherer(opts)
	if err != nil {
		t.Error(err)
	}

	if gatherer.State() != ICEGathererStateNew {
		t.Fatalf("Expected gathering state new")
	}

	gatherFinished := make(chan struct{})
	gatherer.OnLocalCandidate(func(i *ICECandidate) {
		if i == nil {
			close(gatherFinished)
		}
	})

	if err = gatherer.Gather(); err != nil {
		t.Error(err)
	}

	<-gatherFinished

	params, err := gatherer.GetLocalParameters()
	if err != nil {
		t.Error(err)
	}

	if len(params.UsernameFragment) == 0 ||
		len(params.Password) == 0 {
		t.Fatalf("Empty local username or password frag")
	}

	candidates, err := gatherer.GetLocalCandidates()
	if err != nil {
		t.Error(err)
	}

	if len(candidates) == 0 {
		t.Fatalf("No candidates gathered")
	}

	assert.NoError(t, gatherer.Close())
}

func TestICEGather_mDNSCandidateGathering(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	s := SettingEngine{}
	s.GenerateMulticastDNSCandidates(true)

	gatherer, err := NewAPI(WithSettingEngine(s)).NewICEGatherer(ICEGatherOptions{})
	if err != nil {
		t.Error(err)
	}

	gotMulticastDNSCandidate, resolveFunc := context.WithCancel(context.Background())
	gatherer.OnLocalCandidate(func(c *ICECandidate) {
		if c != nil && strings.HasSuffix(c.Address, ".local") {
			resolveFunc()
		}
	})

	assert.NoError(t, gatherer.Gather())

	<-gotMulticastDNSCandidate.Done()
	assert.NoError(t, gatherer.Close())
}
