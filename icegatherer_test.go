// +build !js

package webrtc

import (
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/pion/transport/test"
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

	gatherer, err := NewICEGatherer(0, 0, nil, nil, nil, nil, nil, nil, nil, logging.NewDefaultLoggerFactory(), false, false, nil, opts)
	if err != nil {
		t.Error(err)
	}

	if gatherer.State() != ICEGathererStateNew {
		t.Fatalf("Expected gathering state new")
	}

	err = gatherer.Gather()
	if err != nil {
		t.Error(err)
	}

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

	err = gatherer.Close()
	if err != nil {
		t.Error(err)
	}
}

func TestICEGather_LocalCandidateOrder(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	opts := ICEGatherOptions{
		ICEServers: []ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}

	to := time.Second
	gatherer, err := NewICEGatherer(10000, 10010, &to, &to, &to, &to, &to, &to, &to, logging.NewDefaultLoggerFactory(), false, false, []NetworkType{NetworkTypeUDP4}, opts)
	if err != nil {
		t.Error(err)
	}

	if gatherer.State() != ICEGathererStateNew {
		t.Fatalf("Expected gathering state new")
	}

	for i := 0; i < 10; i++ {
		candidate := make(chan *ICECandidate)
		gatherer.OnLocalCandidate(func(c *ICECandidate) {
			candidate <- c
		})

		if err := gatherer.SignalCandidates(); err != nil {
			t.Error(err)
		}
		endGathering := false

	L:
		for {
			select {
			case c := <-candidate:
				if c == nil {
					endGathering = true
				} else if endGathering {
					t.Error("Received a candidate after the last candidate")
					break L
				}
			case <-time.After(100 * time.Millisecond):
				if !endGathering {
					t.Error("Timed out before receiving the last candidate")
				}
				break L
			}
		}
	}

	if err := gatherer.Close(); err != nil {
		t.Error(err)
	}
}
