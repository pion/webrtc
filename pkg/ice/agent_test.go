package ice

import (
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
