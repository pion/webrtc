// +build !js

package webrtc

import (
	"testing"
	"time"

	"github.com/pion/ice"
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

	gatherer, err := NewICEGatherer(0, 0, nil, nil, nil, nil, nil, nil, nil, logging.NewDefaultLoggerFactory(), false, false, nil, func(string) bool { return true }, nil, "", opts)
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
	gatherer, err := NewICEGatherer(10000, 10010, &to, &to, &to, &to, &to, &to, &to, logging.NewDefaultLoggerFactory(), false, false, []NetworkType{NetworkTypeUDP4}, func(string) bool { return true }, nil, "", opts)
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

func TestNewICEGatherer_NAT1To1IP(t *testing.T) {
	t.Run("1:1 NAT with host", func(t *testing.T) {
		opts := ICEGatherOptions{
			ICEServers: []ICEServer{},
		}

		gatherer, err := NewICEGatherer(
			0, 0, nil, nil, nil, nil, nil, nil, nil,
			logging.NewDefaultLoggerFactory(),
			false, false, nil, func(string) bool { return true },
			[]string{"1.2.3.4"}, "host", // <---- testing here
			opts)
		if err != nil {
			t.Error(err)
		}

		if len(gatherer.nat1To1IPs) != 1 {
			t.Fatal("unexpected nat1To1IPs length")
		}
		if gatherer.nat1To1IPs[0] != "1.2.3.4" {
			t.Fatal("unexpected nat1To1IPs value")
		}
		if gatherer.nat1To1IPCandidateType != ice.CandidateTypeHost {
			t.Fatal("unexpected nat1To1IPs value")
		}

		err = gatherer.Close()
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("1:1 NAT with srflx", func(t *testing.T) {
		opts := ICEGatherOptions{
			ICEServers: []ICEServer{},
		}

		gatherer, err := NewICEGatherer(
			0, 0, nil, nil, nil, nil, nil, nil, nil,
			logging.NewDefaultLoggerFactory(),
			false, false, nil, func(string) bool { return true },
			[]string{"4.5.6.7"}, "srflx", // <---- testing here
			opts)
		if err != nil {
			t.Error(err)
		}

		if len(gatherer.nat1To1IPs) != 1 {
			t.Fatal("unexpected nat1To1IPs length")
		}
		if gatherer.nat1To1IPs[0] != "4.5.6.7" {
			t.Fatal("unexpected nat1To1IPs value")
		}
		if gatherer.nat1To1IPCandidateType != ice.CandidateTypeServerReflexive {
			t.Fatal("unexpected nat1To1IPs value")
		}

		err = gatherer.Close()
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("1:1 NAT with invalid candidate type", func(t *testing.T) {
		opts := ICEGatherOptions{
			ICEServers: []ICEServer{},
		}

		gatherer, err := NewICEGatherer(
			0, 0, nil, nil, nil, nil, nil, nil, nil,
			logging.NewDefaultLoggerFactory(),
			false, false, nil, func(string) bool { return true },
			[]string{"6.6.6.6"}, "prflx", // <---- testing here
			opts)
		if err != nil {
			t.Error(err)
		}

		if len(gatherer.nat1To1IPs) != 1 {
			t.Fatal("unexpected nat1To1IPs length")
		}
		if gatherer.nat1To1IPs[0] != "6.6.6.6" {
			t.Fatal("unexpected nat1To1IPs value")
		}
		if gatherer.nat1To1IPCandidateType != ice.CandidateTypeUnspecified {
			t.Fatal("unexpected nat1To1IPs value")
		}

		err = gatherer.Close()
		if err != nil {
			t.Error(err)
		}
	})
}
