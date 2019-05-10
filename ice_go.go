// +build !js

package webrtc

// NewICEGatherer creates a new NewICEGatherer.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewICEGatherer(opts ICEGatherOptions) (*ICEGatherer, error) {
	return NewICEGatherer(
		api.settingEngine.ephemeralUDP.PortMin,
		api.settingEngine.ephemeralUDP.PortMax,
		api.settingEngine.timeout.ICEConnection,
		api.settingEngine.timeout.ICEKeepalive,
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
