// +build !js

package webrtc

import (
	"testing"
	"time"

	"github.com/pions/transport/test"
	"github.com/pions/webrtc/internal/util"
)

func TestDataChannel_ORTCE2E(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	stackA, stackB, err := newORTCPair()
	if err != nil {
		t.Fatal(err)
	}

	awaitSetup := make(chan struct{})
	awaitString := make(chan struct{})
	awaitBinary := make(chan struct{})
	stackB.sctp.OnDataChannel(func(d *DataChannel) {
		close(awaitSetup)

		d.OnMessage(func(msg DataChannelMessage) {
			if msg.IsString {
				close(awaitString)
			} else {
				close(awaitBinary)
			}
		})
	})

	err = signalORTCPair(stackA, stackB)
	if err != nil {
		t.Fatal(err)
	}

	dcParams := &DataChannelParameters{
		Label: "Foo",
		ID:    1,
	}
	channelA, err := stackA.api.NewDataChannel(stackA.sctp, dcParams)
	if err != nil {
		t.Fatal(err)
	}

	<-awaitSetup

	err = channelA.SendText("ABC")
	if err != nil {
		t.Fatal(err)
	}
	err = channelA.Send([]byte("ABC"))
	if err != nil {
		t.Fatal(err)
	}
	<-awaitString
	<-awaitBinary

	err = stackA.close()
	if err != nil {
		t.Fatal(err)
	}

	err = stackB.close()
	if err != nil {
		t.Fatal(err)
	}
}

type testORTCStack struct {
	api      *API
	gatherer *ICEGatherer
	ice      *ICETransport
	dtls     *DTLSTransport
	sctp     *SCTPTransport
}

func (s *testORTCStack) setSignal(sig *testORTCSignal, isOffer bool) error {
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

	// Start the DTLS transport
	err = s.dtls.Start(sig.DTLSParameters)
	if err != nil {
		return err
	}

	// Start the SCTP transport
	err = s.sctp.Start(sig.SCTPCapabilities)
	if err != nil {
		return err
	}

	return nil
}

func (s *testORTCStack) getSignal() (*testORTCSignal, error) {
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

	dtlsParams := s.dtls.GetLocalParameters()

	sctpCapabilities := s.sctp.GetCapabilities()

	return &testORTCSignal{
		ICECandidates:    iceCandidates,
		ICEParameters:    iceParams,
		DTLSParameters:   dtlsParams,
		SCTPCapabilities: sctpCapabilities,
	}, nil
}

func (s *testORTCStack) close() error {
	var closeErrs []error

	if err := s.sctp.Stop(); err != nil {
		closeErrs = append(closeErrs, err)
	}

	if err := s.ice.Stop(); err != nil {
		closeErrs = append(closeErrs, err)
	}

	return util.FlattenErrs(closeErrs)
}

type testORTCSignal struct {
	ICECandidates    []ICECandidate   `json:"iceCandidates"`
	ICEParameters    ICEParameters    `json:"iceParameters"`
	DTLSParameters   DTLSParameters   `json:"dtlsParameters"`
	SCTPCapabilities SCTPCapabilities `json:"sctpCapabilities"`
}

func newORTCPair() (stackA *testORTCStack, stackB *testORTCStack, err error) {
	sa, err := newORTCStack()
	if err != nil {
		return nil, nil, err
	}

	sb, err := newORTCStack()
	if err != nil {
		return nil, nil, err
	}

	return sa, sb, nil
}

func newORTCStack() (*testORTCStack, error) {
	// Create an API object
	api := NewAPI()

	// Create the ICE gatherer
	gatherer, err := api.NewICEGatherer(ICEGatherOptions{})
	if err != nil {
		return nil, err
	}

	// Construct the ICE transport
	ice := api.NewICETransport(gatherer)

	// Construct the DTLS transport
	dtls, err := api.NewDTLSTransport(ice, nil)
	if err != nil {
		return nil, err
	}

	// Construct the SCTP transport
	sctp := api.NewSCTPTransport(dtls)

	return &testORTCStack{
		api:      api,
		gatherer: gatherer,
		ice:      ice,
		dtls:     dtls,
		sctp:     sctp,
	}, nil
}

func signalORTCPair(stackA *testORTCStack, stackB *testORTCStack) error {
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
