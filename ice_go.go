// +build !js

package webrtc

import (
	"github.com/pion/sdp/v2"
	"github.com/pion/webrtc/v2/pkg/ice"
)

//go:generate go run internal/tools/gen/genaliasdocs.go -pkg "./pkg/ice" -build-tags "!js" $GOFILE

type (

	// ICETransport allows an application access to information about the ICE
	// transport over which packets are sent and received.
	ICETransport = ice.Transport

	// ICEGatherer gathers local host, server reflexive and relay
	// candidates, as well as enabling the retrieval of local Interactive
	// Connectivity Establishment (ICE) parameters which can be
	// exchanged in signaling.
	ICEGatherer = ice.Gatherer
)

var (

	// NewICEGatherer creates a new Gatherer.
	NewICEGatherer = ice.NewGatherer

	// NewICETransport creates a new NewICETransport.
	NewICETransport = ice.NewTransport
)

// NewICEGatherer creates a new NewICEGatherer.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewICEGatherer(opts ICEGatherOptions) (*ICEGatherer, error) {
	return NewICEGatherer(
		api.settingEngine.ephemeralUDP.PortMin,
		api.settingEngine.ephemeralUDP.PortMax,
		api.settingEngine.timeout.ICEConnection,
		api.settingEngine.timeout.ICEKeepalive,
		api.settingEngine.timeout.ICECandidateSelectionTimeout,
		api.settingEngine.timeout.ICEHostAcceptanceMinWait,
		api.settingEngine.timeout.ICESrflxAcceptanceMinWait,
		api.settingEngine.timeout.ICEPrflxAcceptanceMinWait,
		api.settingEngine.timeout.ICERelayAcceptanceMinWait,
		api.settingEngine.LoggerFactory,
		api.settingEngine.candidates.ICENetworkTypes,
		opts,
	)
}

// NewICETransport creates a new NewICETransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewICETransport(gatherer *ICEGatherer) *ICETransport {
	return NewICETransport(gatherer, api.settingEngine.LoggerFactory)
}

func newICECandidateFromSDP(c sdp.ICECandidate) (ICECandidate, error) {
	typ, err := NewICECandidateType(c.Typ)
	if err != nil {
		return ICECandidate{}, err
	}
	protocol, err := NewICEProtocol(c.Protocol)
	if err != nil {
		return ICECandidate{}, err
	}
	return ICECandidate{
		Foundation:     c.Foundation,
		Priority:       c.Priority,
		IP:             c.IP,
		Protocol:       protocol,
		Port:           c.Port,
		Component:      c.Component,
		Typ:            typ,
		RelatedAddress: c.RelatedAddress,
		RelatedPort:    c.RelatedPort,
	}, nil
}

func iceCandidateToSDP(c ICECandidate) sdp.ICECandidate {
	return sdp.ICECandidate{
		Foundation:     c.Foundation,
		Priority:       c.Priority,
		IP:             c.IP,
		Protocol:       c.Protocol.String(),
		Port:           c.Port,
		Component:      c.Component,
		Typ:            c.Typ.String(),
		RelatedAddress: c.RelatedAddress,
		RelatedPort:    c.RelatedPort,
	}
}
