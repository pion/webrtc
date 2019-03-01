package webrtc

import (
	"github.com/pions/webrtc/internal/ice"
)

// Configuration defines a set of parameters to configure how the
// peer-to-peer communication via PeerConnection is established or
// re-established.
type Configuration struct {
	// ICEServers defines a slice describing servers available to be used by
	// ICE, such as STUN and TURN servers.
	ICEServers []ICEServer

	// ICETransportPolicy indicates which candidates the ICEAgent is allowed
	// to use.
	ICETransportPolicy ICETransportPolicy

	// BundlePolicy indicates which media-bundling policy to use when gathering
	// ICE candidates.
	BundlePolicy BundlePolicy

	// RTCPMuxPolicy indicates which rtcp-mux policy to use when gathering ICE
	// candidates.
	RTCPMuxPolicy RTCPMuxPolicy

	// PeerIdentity sets the target peer identity for the PeerConnection.
	// The PeerConnection will not establish a connection to a remote peer
	// unless it can be successfully authenticated with the provided name.
	PeerIdentity string

	// Certificates describes a set of certificates that the PeerConnection
	// uses to authenticate. Valid values for this parameter are created
	// through calls to the GenerateCertificate function. Although any given
	// DTLS connection will use only one certificate, this attribute allows the
	// caller to provide multiple certificates that support different
	// algorithms. The final certificate will be selected based on the DTLS
	// handshake, which establishes which certificates are allowed. The
	// PeerConnection implementation selects which of the certificates is
	// used for a given connection; how certificates are selected is outside
	// the scope of this specification. If this value is absent, then a default
	// set of certificates is generated for each PeerConnection instance.
	Certificates []Certificate

	// ICECandidatePoolSize describes the size of the prefetched ICE pool.
	ICECandidatePoolSize uint8
}

func (c Configuration) getICEServers() (*[]*ice.URL, error) {
	var iceServers []*ice.URL
	for _, server := range c.ICEServers {
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
