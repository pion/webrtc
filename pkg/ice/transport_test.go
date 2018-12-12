package ice

import (
	"context"
	"testing"
	"time"

	"github.com/pions/transport/test"
)

func TestStressDuplex(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	// Check for leaking routines
	report := test.CheckRoutines(t)
	defer report()

	// Run the test
	stressDuplex(t)
}

func stressDuplex(t *testing.T) {
	ca, cb := pipe()

	defer func() {
		err := ca.Close()
		if err != nil {
			t.Fatal(err)
		}
		err = cb.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	opt := test.Options{
		MsgSize:  10,
		MsgCount: 1, // Order not reliable due to UDP & potentially multiple candidate pairs.
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

func copyCandidate(orig *Candidate) *Candidate {
	c := &Candidate{
		Type:        orig.Type,
		NetworkType: orig.NetworkType,
		IP:          orig.IP,
		Port:        orig.Port,
	}

	if orig.RelatedAddress != nil {
		c.RelatedAddress = &CandidateRelatedAddress{
			Address: orig.RelatedAddress.Address,
			Port:    orig.RelatedAddress.Port,
		}
	}

	return c
}

func onConnected() (func(ConnectionState), chan struct{}) {
	done := make(chan struct{})
	return func(state ConnectionState) {
		if state == ConnectionStateConnected {
			close(done)
		}
	}, done
}
