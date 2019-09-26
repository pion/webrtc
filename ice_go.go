// +build !js

package webrtc

import "github.com/pion/sdp/v2"

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
		api.settingEngine.candidates.ICETrickle,
		api.settingEngine.candidates.ICELite,
		api.settingEngine.candidates.ICENetworkTypes,
		api.settingEngine.candidates.InterfaceFilter,
		api.settingEngine.candidates.NAT1To1IPs,
		api.settingEngine.candidates.NAT1To1IPCandidateType,
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
		Address:        c.Address,
		Protocol:       protocol,
		Port:           c.Port,
		Component:      c.Component,
		Typ:            typ,
		RelatedAddress: c.RelatedAddress,
		RelatedPort:    c.RelatedPort,
	}, nil
}
