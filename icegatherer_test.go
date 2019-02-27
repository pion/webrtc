package webrtc

import (
	"testing"
	"time"

	"github.com/pions/webrtc/pkg/ice"

	"github.com/pions/transport/test"
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

	gatherer, err := NewICEGatherer(opts, nil)
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

func TestNewICEGatherer_BadPort(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	_, err := NewICEGatherer(ICEGatherOptions{}, &ICEAgentOptions{
		PortMin: 3000,
		PortMax: 2000,
	})
	if got, want := err, ice.ErrPort; got != want {
		t.Fatalf("NewICEGatherer with invalid port spec: err=%v, want %v", got, want)
	}
}
