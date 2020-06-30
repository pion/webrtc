// +build !js
// +build quic

package webrtc

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/pion/quic"
	"github.com/pion/transport/test"
	"github.com/pion/webrtc/v3/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestQUICTransport_E2E(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	// Check how we can make sure quic-go closes without leaking
	report := test.CheckRoutines(t)
	defer report()

	stackA, stackB, err := newQuicPair()
	if err != nil {
		t.Fatal(err)
	}

	awaitSetup := make(chan struct{})
	dataAgot := make(chan []byte)
	dataBgot := make(chan []byte)
	stackB.quic.OnBidirectionalStream(func(stream *quic.BidirectionalStream) {
		go quicReadLoop(stream, dataBgot) // Read to pull incoming messages

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

	go quicReadLoop(stream, dataAgot) // Read to pull incoming messages

	go func() {
		for d := range dataAgot {
			t.Errorf("unexpected data: %q", d)
		}
	}()

	testData := bytes.Repeat([]byte("Hello"), 128)
	count := 1024

	var bufSent, bufGot bytes.Buffer

	// read side
	done := make(chan struct{})
	go func() {
		<-awaitSetup
		t.Log("connection established")

		for rx := range dataBgot {
			_, werr := bufGot.Write(rx)
			assert.NoError(t, werr)
		}
		close(done)
	}()

	// sent side
	for i := 0; i < count; i++ {
		var buf [2]byte
		binary.BigEndian.PutUint16(buf[:], uint16(i))
		msg := append(buf[:], testData...)
		_, werr := bufSent.Write(msg)
		assert.NoError(t, werr)

		data := quic.StreamWriteParameters{Data: msg}
		if i == count-1 {
			data.Finished = true
		}
		werr = stream.Write(data)
		if werr != nil {
			t.Fatal(werr)
		}
	}

	<-done
	t.Log("read all data from stream")

	assert.Equal(t, bufSent.Len(), count*(len(testData)+2))
	assert.Equal(t, bufSent.Len(), bufGot.Len())

	err = stackA.close()
	if err != nil {
		t.Fatal(err)
	}

	err = stackB.close()
	if err != nil {
		t.Fatal(err)
	}
}

func quicReadLoop(s *quic.BidirectionalStream, got chan<- []byte) {
	defer close(got)
	for {
		buffer := make([]byte, 4098)
		res, err := s.ReadInto(buffer)
		if res.Amount > 0 {
			got <- buffer[:res.Amount]
		}
		if err != nil || res.Finished {
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
	gatherFinished := make(chan struct{})
	s.gatherer.OnLocalCandidate(func(i *ICECandidate) {
		if i == nil {
			close(gatherFinished)
			return
		}
	})
	err := s.gatherer.Gather()
	if err != nil {
		return nil, err
	}

	<-gatherFinished
	iceCandidates, err := s.gatherer.GetLocalCandidates()
	if err != nil {
		return nil, err
	}

	iceParams, err := s.gatherer.GetLocalParameters()
	if err != nil {
		return nil, err
	}

	quicParams, err := s.quic.GetLocalParameters()
	if err != nil {
		return nil, err
	}

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

	return util.FlattenErrs(closeErrs)
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

	return util.FlattenErrs(closeErrs)
}
