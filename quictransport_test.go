package webrtc

import (
	"testing"
	"time"

	"github.com/pions/quic"
	"github.com/pions/transport/test"
)

func TestQUICTransport_E2E(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	// TODO: Check how we can make sure quic-go closes without leaking
	// report := test.CheckRoutines(t)
	// defer report()

	stackA, stackB, err := newQuicPair()
	if err != nil {
		t.Fatal(err)
	}

	awaitSetup := make(chan struct{})
	stackB.quic.OnBidirectionalStream(func(stream *quic.BidirectionalStream) {
		go quicReadLoop(stream) // Read to pull incoming messages

		close(awaitSetup)
	})

	err = signalQuicPair(stackA, stackB)
	if err != nil {
		t.Fatal(err)
	}

	stream, err := stackA.quic.CreateBidirectionalStream()
	if err != nil {
		t.Fatal(err)
	}

	go quicReadLoop(stream) // Read to pull incoming messages

	// Write to open stream
	data := quic.StreamWriteParameters{
		Data: []byte("Hello"),
	}
	err = stream.Write(data)
	if err != nil {
		t.Fatal(err)
	}

	<-awaitSetup

	err = stackA.close()
	if err != nil {
		t.Fatal(err)
	}

	err = stackB.close()
	if err != nil {
		t.Fatal(err)
	}
}

func quicReadLoop(s *quic.BidirectionalStream) {
	for {
		buffer := make([]byte, 15)
		_, err := s.ReadInto(buffer)
		if err != nil {
			return
		}
	}
}

type testQuicStack struct {
	gatherer *ICEGatherer
	ice      *ICETransport
	quic     *QUICTransport
	api      *API
}

func (s *testQuicStack) setSignal(sig *testQuicSignal, isOffer bool) error {
	iceRole := ICERoleControlled
	if isOffer {
		iceRole = ICERoleControlling
	}

	err := s.ice.SetRemoteCandidates(sig.ICECandidates)
	if err != nil {
		return err
	}

	// Start the ICE transport
	err = s.ice.Start(nil, sig.ICEParameters, &iceRole)
	if err != nil {
		return err
	}

	// Start the Quic transport
	err = s.quic.Start(sig.QuicParameters)
	if err != nil {
		return err
	}

	return nil
}

func (s *testQuicStack) getSignal() (*testQuicSignal, error) {
	// Gather candidates
	err := s.gatherer.Gather()
	if err != nil {
		return nil, err
	}

	iceCandidates, err := s.gatherer.GetLocalCandidates()
	if err != nil {
		return nil, err
	}

	iceParams, err := s.gatherer.GetLocalParameters()
	if err != nil {
		return nil, err
	}

	quicParams := s.quic.GetLocalParameters()

	return &testQuicSignal{
		ICECandidates:  iceCandidates,
		ICEParameters:  iceParams,
		QuicParameters: quicParams,
	}, nil
}

func (s *testQuicStack) close() error {
	var closeErrs []error

	if err := s.quic.Stop(quic.TransportStopInfo{}); err != nil {
		closeErrs = append(closeErrs, err)
	}

	if err := s.ice.Stop(); err != nil {
		closeErrs = append(closeErrs, err)
	}

	return flattenErrs(closeErrs)
}

type testQuicSignal struct {
	ICECandidates  []ICECandidate `json:"iceCandidates"`
	ICEParameters  ICEParameters  `json:"iceParameters"`
	QuicParameters QUICParameters `json:"quicParameters"`
}

func newQuicPair() (stackA *testQuicStack, stackB *testQuicStack, err error) {
	sa, err := newQuicStack()
	if err != nil {
		return nil, nil, err
	}

	sb, err := newQuicStack()
	if err != nil {
		return nil, nil, err
	}

	return sa, sb, nil
}

func newQuicStack() (*testQuicStack, error) {
	api := NewAPI()
	// Create the ICE gatherer
	gatherer, err := api.NewICEGatherer(ICEGatherOptions{})
	if err != nil {
		return nil, err
	}

	// Construct the ICE transport
	ice := api.NewICETransport(gatherer)

	// Construct the Quic transport
	qt, err := api.NewQUICTransport(ice, nil)
	if err != nil {
		return nil, err
	}

	return &testQuicStack{
		api:      api,
		gatherer: gatherer,
		ice:      ice,
		quic:     qt,
	}, nil
}

func signalQuicPair(stackA *testQuicStack, stackB *testQuicStack) error {
	sigA, err := stackA.getSignal()
	if err != nil {
		return err
	}
	sigB, err := stackB.getSignal()
	if err != nil {
		return err
	}

	a := make(chan error)
	b := make(chan error)

	go func() {
		a <- stackB.setSignal(sigA, false)
	}()

	go func() {
		b <- stackA.setSignal(sigB, true)
	}()

	errA := <-a
	errB := <-b

	closeErrs := []error{errA, errB}

	return flattenErrs(closeErrs)
}
