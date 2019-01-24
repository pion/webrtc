package webrtc

import (
	"testing"
	"time"

	"github.com/pions/transport/test"
)

func TestNewRTCIceGatherer_Success(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	opts := RTCIceGatherOptions{
		ICEServers: []RTCIceServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}

	api := NewAPI()

	gatherer, err := api.NewRTCIceGatherer(opts)
	if err != nil {
		t.Error(err)
	}

	if gatherer.State() != RTCIceGathererStateNew {
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
