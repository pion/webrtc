package ice

import (
	"context"
	"fmt"
	"testing"

	"github.com/pions/webrtc/internal/transport/test"
)

func Benchmark(b *testing.B) {
	ca, cb := pipe()

	b.ResetTimer()

	opt := test.Options{
		MsgSize:  128,
		MsgCount: b.N,
	}

	err := test.Stress(ca, cb, opt)
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
	base := CandidateBase{
		Protocol: orig.GetBase().Protocol,
		Address:  orig.GetBase().Address,
		Port:     orig.GetBase().Port,
	}

	switch v := orig.(type) {
	case *CandidateHost:
		return &CandidateHost{CandidateBase: base}
	case *CandidateSrflx:
		return &CandidateSrflx{CandidateBase: base,
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
