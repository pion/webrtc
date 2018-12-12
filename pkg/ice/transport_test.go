package ice

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pions/transport/test"
)

func TestStressDuplex(t *testing.T) {
	lim := test.TimeOut(time.Second * 5)
	defer lim.Stop()

	ca, cb := pipe()

	defer func() {
		err := ca.Close()
		check(err)
		err = cb.Close()
		check(err)
	}()

	opt := test.Options{
		MsgSize:  2048,
		MsgCount: 1, // Can't rely on UDP message order in CI
	}

	err := test.StressDuplex(ca, cb, opt)
	if err != nil {
		t.Fatal(err)
	}
}

func Benchmark(b *testing.B) {
	ca, cb := pipe()
	defer func() {
		err := ca.Close()
		check(err)
		err = cb.Close()
		check(err)
	}()

	b.ResetTimer()

	opt := test.Options{
		MsgSize:  128,
		MsgCount: b.N,
	}

	err := test.StressDuplex(ca, cb, opt)
	check(err)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func pipe() (*Conn, *Conn) {
	var urls []*URL

	aNotifier, aConnected := onConnected()
	bNotifier, bConnected := onConnected()

	aAgent := NewAgent(urls, aNotifier)
	bAgent := NewAgent(urls, bNotifier)

	// Manual signaling
	aUfrag, aPwd := aAgent.GetLocalUserCredentials()
	bUfrag, bPwd := bAgent.GetLocalUserCredentials()

	candidates, err := aAgent.GetLocalCandidates()
	check(err)
	for _, c := range candidates {
		check(bAgent.AddRemoteCandidate(copyCandidate(c)))
	}

	candidates, err = bAgent.GetLocalCandidates()
	check(err)
	for _, c := range candidates {
		check(aAgent.AddRemoteCandidate(copyCandidate(c)))
	}

	accepted := make(chan struct{})
	var aConn *Conn

	go func() {
		var acceptErr error
		aConn, acceptErr = aAgent.Accept(context.TODO(), bUfrag, bPwd)
		check(acceptErr)
		close(accepted)
	}()

	bConn, err := bAgent.Dial(context.TODO(), aUfrag, aPwd)
	check(err)

	// Ensure accepted
	<-accepted

	// Ensure pair selected
	// Note: this assumes ConnectionStateConnected is thrown after selecting the final pair
	<-aConnected
	<-bConnected

	return aConn, bConn
}

func copyCandidate(orig Candidate) Candidate {
	// Quiet go vet kvetching about mutex copying
	base := func() CandidateBase {
		return CandidateBase{
			NetworkType: orig.GetBase().NetworkType,
			IP:          orig.GetBase().IP,
			Port:        orig.GetBase().Port,
		}
	}

	switch v := orig.(type) {
	case *CandidateHost:
		return &CandidateHost{
			CandidateBase: base(),
		}
	case *CandidateSrflx:
		return &CandidateSrflx{CandidateBase: base(),
			RelatedAddress: v.RelatedAddress,
			RelatedPort:    v.RelatedPort}
	default:
		fmt.Printf("I don't know about type %T!\n", v)
	}
	return nil
}

func onConnected() (func(ConnectionState), chan struct{}) {
	done := make(chan struct{})
	return func(state ConnectionState) {
		if state == ConnectionStateConnected {
			close(done)
		}
	}, done
}
