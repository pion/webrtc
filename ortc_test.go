// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"github.com/pion/webrtc/v3/internal/util"
)

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
	gatherFinished := make(chan struct{})
	s.gatherer.OnLocalCandidate(func(i *ICECandidate) {
		if i == nil {
			close(gatherFinished)
		}
	})

	if err := s.gatherer.Gather(); err != nil {
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

	dtlsParams, err := s.dtls.GetLocalParameters()
	if err != nil {
		return nil, err
	}

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
	ICECandidates    []ICECandidate
	ICEParameters    ICEParameters
	DTLSParameters   DTLSParameters
	SCTPCapabilities SCTPCapabilities
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
