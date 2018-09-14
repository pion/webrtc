package webrtc

import (
	"github.com/pions/webrtc/pkg/ice"
)

// RTCConfiguration defines a set of parameters to configure how the
// peer-to-peer communication via RTCPeerConnection is established or
// re-established.
type RTCConfiguration struct {
	// IceServers defines a slice describing servers available to be used by
	// ICE, such as STUN and TURN servers.
	IceServers []RTCIceServer

	// IceTransportPolicy indicates which candidates the IceAgent is allowed
	// to use.
	IceTransportPolicy RTCIceTransportPolicy

	// BundlePolicy indicates which media-bundling policy to use when gathering
	// ICE candidates.
	BundlePolicy RTCBundlePolicy

	// RtcpMuxPolicy indicates which rtcp-mux policy to use when gathering ICE
	// candidates.
	RtcpMuxPolicy RTCRtcpMuxPolicy

	// PeerIdentity sets the target peer identity for the RTCPeerConnection.
	// The RTCPeerConnection will not establish a connection to a remote peer
	// unless it can be successfully authenticated with the provided name.
	PeerIdentity string

	// Certificates describes a set of certificates that the RTCPeerConnection
	// uses to authenticate. Valid values for this parameter are created
	// through calls to the GenerateCertificate function. Although any given
	// DTLS connection will use only one certificate, this attribute allows the
	// caller to provide multiple certificates that support different
	// algorithms. The final certificate will be selected based on the DTLS
	// handshake, which establishes which certificates are allowed. The
	// RTCPeerConnection implementation selects which of the certificates is
	// used for a given connection; how certificates are selected is outside
	// the scope of this specification. If this value is absent, then a default
	// set of certificates is generated for each RTCPeerConnection instance.
	Certificates []RTCCertificate

	// IceCandidatePoolSize describes the size of the prefetched ICE pool.
	IceCandidatePoolSize uint8
}

func (c RTCConfiguration) getIceServers() (*[]*ice.URL, error) {
	var iceServers []*ice.URL
	for _, server := range c.IceServers {
		for _, rawURL := range server.URLs {
			url, err := ice.ParseURL(rawURL)
			if err != nil {
				return nil, err
			}
			iceServers = append(iceServers, url)
		}
	}
	return &iceServers, nil
}
