package webrtc

// RTCConfiguration contains RTCPeerConfiguration options
type RTCConfiguration struct {
	// IceServers holds multiple RTCIceServer instances, each describing one server which may be used by the ICE agent;
	// these are typically STUN and/or TURN servers. If this isn't specified, the ICE agent may choose to use its own
	// ICE servers; otherwise, the connection attempt will be made with no STUN or TURN server available, which limits
	// the connection to local peers.
	IceServers           []RTCIceServer
	IceTransportPolicy   RTCIceTransportPolicy
	BundlePolicy         RTCBundlePolicy
	RtcpMuxPolicy        RTCRtcpMuxPolicy
	PeerIdentity         string
	Certificates         []RTCCertificate
	IceCandidatePoolSize uint8
}
