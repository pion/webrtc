// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/transport/v3/test"
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
	assert.NoError(t, err)
	assert.Equal(t, ICEGathererStateNew, gatherer.State())

	gatherFinished := make(chan struct{})
	gatherer.OnLocalCandidate(func(i *ICECandidate) {
		if i == nil {
			close(gatherFinished)
		}
	})

	assert.NoError(t, gatherer.Gather())

	<-gatherFinished

	params, err := gatherer.GetLocalParameters()
	assert.NoError(t, err)

	assert.NotEmpty(t, params.UsernameFragment, "Empty local username frag")
	assert.NotEmpty(t, params.Password, "Empty local password")

	candidates, err := gatherer.GetLocalCandidates()
	assert.NoError(t, err)
	assert.NotEmpty(t, candidates, "No candidates gathered")

	assert.NoError(t, gatherer.Close())
}

func TestICEGather_mDNSCandidateGathering(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	s := SettingEngine{}
	s.SetICEMulticastDNSMode(ice.MulticastDNSModeQueryAndGather)

	gatherer, err := NewAPI(WithSettingEngine(s)).NewICEGatherer(ICEGatherOptions{})
	assert.NoError(t, err)

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

func TestICEGatherer_AlreadyClosed(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	opts := ICEGatherOptions{
		ICEServers: []ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}

	t.Run("Gather", func(t *testing.T) {
		gatherer, err := NewAPI().NewICEGatherer(opts)
		assert.NoError(t, err)

		err = gatherer.createAgent()
		assert.NoError(t, err)

		err = gatherer.Close()
		assert.NoError(t, err)

		err = gatherer.Gather()
		assert.ErrorIs(t, err, errICEAgentNotExist)
	})

	t.Run("GetLocalParameters", func(t *testing.T) {
		gatherer, err := NewAPI().NewICEGatherer(opts)
		assert.NoError(t, err)

		err = gatherer.createAgent()
		assert.NoError(t, err)

		err = gatherer.Close()
		assert.NoError(t, err)

		_, err = gatherer.GetLocalParameters()
		assert.ErrorIs(t, err, errICEAgentNotExist)
	})

	t.Run("GetLocalCandidates", func(t *testing.T) {
		gatherer, err := NewAPI().NewICEGatherer(opts)
		assert.NoError(t, err)

		err = gatherer.createAgent()
		assert.NoError(t, err)

		err = gatherer.Close()
		assert.NoError(t, err)

		_, err = gatherer.GetLocalCandidates()
		assert.ErrorIs(t, err, errICEAgentNotExist)
	})
}

func TestNewICEGathererSetMediaStreamIdentification(t *testing.T) { //nolint:cyclop
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	opts := ICEGatherOptions{
		ICEServers: []ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}

	gatherer, err := NewAPI().NewICEGatherer(opts)
	assert.NoError(t, err)

	expectedMid := "5"
	expectedMLineIndex := uint16(1)

	gatherer.setMediaStreamIdentification(expectedMid, expectedMLineIndex)

	assert.Equal(t, ICEGathererStateNew, gatherer.State())

	gatherFinished := make(chan struct{})
	gatherer.OnLocalCandidate(func(i *ICECandidate) {
		if i == nil {
			close(gatherFinished)
		} else {
			assert.Equal(t, expectedMid, i.SDPMid)
			assert.Equal(t, expectedMLineIndex, i.SDPMLineIndex)
		}
	})

	assert.NoError(t, gatherer.Gather())
	<-gatherFinished

	params, err := gatherer.GetLocalParameters()
	assert.NoError(t, err)

	assert.NotEmpty(t, params.UsernameFragment, "Empty local username frag")
	assert.NotEmpty(t, params.Password, "Empty local password")

	candidates, err := gatherer.GetLocalCandidates()
	assert.NoError(t, err)
	assert.NotEmpty(t, candidates, "No candidates gathered")

	for _, c := range candidates {
		assert.Equal(t, expectedMid, c.SDPMid)
		assert.Equal(t, expectedMLineIndex, c.SDPMLineIndex)
	}

	assert.NoError(t, gatherer.Close())
}
