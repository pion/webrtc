package webrtc

import (
	"time"

	"github.com/pions/webrtc/pkg/ice"
)

// RTCICECredentialType indicates the type of credentials used to connect to an ICE server
type RTCICECredentialType int

const (
	// RTCICECredentialTypePassword describes username+pasword based credentials
	RTCICECredentialTypePassword RTCICECredentialType = iota + 1
	// RTCICECredentialTypeOauth describes token based credentials
	RTCICECredentialTypeOauth
)

func (t RTCICECredentialType) String() string {
	switch t {
	case RTCICECredentialTypePassword:
		return "password"
	case RTCICECredentialTypeOauth:
		return "oauth"
	default:
		return "Unknown"
	}
}

// RTCCertificate represents a certificate used to authenticate WebRTC communications.
type RTCCertificate struct {
	expires time.Time
	// TODO: Finish during DTLS implementation
}

// Equals determines if two certificates are identical
func (c RTCCertificate) Equals(other RTCCertificate) bool {
	return c.expires == other.expires
}

// RTCICEServer describes a single ICE server, as well as required credentials
type RTCICEServer struct {
	URLs           []string
	Username       string
	Credential     RTCICECredential
	CredentialType RTCICECredentialType
}

// RTCICECredential represents credentials used to connect to an ICE server
// Two types of credentials are supported:
// - Password (type string)
// - Password (type RTCOAuthCredential)
type RTCICECredential interface{}

// RTCOAuthCredential represents OAuth credentials used to connect to an ICE server
type RTCOAuthCredential struct {
	MacKey      string
	AccessToken string
}

// RTCICETransportPolicy defines the ICE candidate policy [JSEP] (section 3.5.3.) used to surface the permitted candidates
type RTCICETransportPolicy int

const (
	// RTCICETransportPolicyRelay indicates only media relay candidates such as candidates passing through a TURN server are used
	RTCICETransportPolicyRelay RTCICETransportPolicy = iota + 1

	// RTCICETransportPolicyAll indicates any type of candidate is used
	RTCICETransportPolicyAll
)

func (t RTCICETransportPolicy) String() string {
	switch t {
	case RTCICETransportPolicyRelay:
		return "relay"
	case RTCICETransportPolicyAll:
		return "all"
	default:
		return "Unknown"
	}
}

// RTCBundlePolicy affects which media tracks are negotiated if the remote endpoint is not bundle-aware, and what ICE candidates are gathered.
type RTCBundlePolicy int

const (

	// RTCRtcpMuxPolicyBalanced indicates to gather ICE candidates for each media type in use (audio, video, and data).
	RTCRtcpMuxPolicyBalanced RTCBundlePolicy = iota + 1

	// RTCRtcpMuxPolicyMaxCompat indicates to gather ICE candidates for each track.
	RTCRtcpMuxPolicyMaxCompat

	// RTCRtcpMuxPolicyMaxBundle indicates to gather ICE candidates for only one track.
	RTCRtcpMuxPolicyMaxBundle
)

func (t RTCBundlePolicy) String() string {
	switch t {
	case RTCRtcpMuxPolicyBalanced:
		return "balanced"
	case RTCRtcpMuxPolicyMaxCompat:
		return "max-compat"
	case RTCRtcpMuxPolicyMaxBundle:
		return "max-bundle"
	default:
		return "Unknown"
	}
}

// RTCRtcpMuxPolicy affects what ICE candidates are gathered to support non-multiplexed RTCP
type RTCRtcpMuxPolicy int

const (
	// RTCRtcpMuxPolicyNegotiate indicates to gather ICE candidates for both RTP and RTCP candidates.
	RTCRtcpMuxPolicyNegotiate RTCRtcpMuxPolicy = iota + 1

	// RTCRtcpMuxPolicyRequire indicates to gather ICE candidates only for RTP and multiplex RTCP on the RTP candidates
	RTCRtcpMuxPolicyRequire
)

func (t RTCRtcpMuxPolicy) String() string {
	switch t {
	case RTCRtcpMuxPolicyNegotiate:
		return "negotiate"
	case RTCRtcpMuxPolicyRequire:
		return "require"
	default:
		return "Unknown"
	}
}

// RTCConfiguration contains RTCPeerConfiguration options
type RTCConfiguration struct {
	// ICEServers holds multiple RTCICEServer instances, each describing one server which may be used by the ICE agent;
	// these are typically STUN and/or TURN servers. If this isn't specified, the ICE agent may choose to use its own ICE servers;
	// otherwise, the connection attempt will be made with no STUN or TURN server available, which limits the connection to local peers.
	ICEServers           []RTCICEServer
	ICETransportPolicy   RTCICETransportPolicy
	BundlePolicy         RTCBundlePolicy
	RtcpMuxPolicy        RTCRtcpMuxPolicy
	PeerIdentity         string
	Certificates         []RTCCertificate
	ICECandidatePoolSize uint8
}

// SetConfiguration updates the configuration of the RTCPeerConnection
func (r *RTCPeerConnection) SetConfiguration(config RTCConfiguration) error {
	err := r.validatePeerIdentity(config)
	if err != nil {
		return err
	}
	err = r.validateCertificates(config)
	if err != nil {
		return err
	}
	err = r.validateBundlePolicy(config)
	if err != nil {
		return err
	}
	err = r.validateRtcpMuxPolicy(config)
	if err != nil {
		return err
	}
	err = r.validateICECandidatePoolSize(config)
	if err != nil {
		return err
	}

	err = r.setICEServers(config)
	if err != nil {
		return err
	}

	r.config = config

	return nil
}

func (r *RTCPeerConnection) validatePeerIdentity(config RTCConfiguration) error {
	current := r.config
	if current.PeerIdentity != "" &&
		config.PeerIdentity != "" &&
		config.PeerIdentity != current.PeerIdentity {
		return &InvalidModificationError{Err: ErrModPeerIdentity}
	}
	return nil
}

func (r *RTCPeerConnection) validateCertificates(config RTCConfiguration) error {
	current := r.config
	if len(current.Certificates) > 0 &&
		len(config.Certificates) > 0 {
		if len(config.Certificates) != len(current.Certificates) {
			return &InvalidModificationError{Err: ErrModCertificates}
		}
		for i, cert := range config.Certificates {
			if !current.Certificates[i].Equals(cert) {
				return &InvalidModificationError{Err: ErrModCertificates}
			}
		}
	}

	now := time.Now()
	for _, cert := range config.Certificates {
		if now.After(cert.expires) {
			return &InvalidAccessError{Err: ErrCertificateExpired}
		}
		// TODO: Check certificate 'origin'
	}
	return nil
}

func (r *RTCPeerConnection) validateBundlePolicy(config RTCConfiguration) error {
	current := r.config
	if config.BundlePolicy != current.BundlePolicy {
		return &InvalidModificationError{Err: ErrModRtcpMuxPolicy}
	}
	return nil
}

func (r *RTCPeerConnection) validateRtcpMuxPolicy(config RTCConfiguration) error {
	current := r.config
	if config.RtcpMuxPolicy != current.RtcpMuxPolicy {
		return &InvalidModificationError{Err: ErrModRtcpMuxPolicy}
	}
	return nil
}

func (r *RTCPeerConnection) validateICECandidatePoolSize(config RTCConfiguration) error {
	current := r.config
	if r.LocalDescription != nil &&
		config.ICECandidatePoolSize != current.ICECandidatePoolSize {
		return &InvalidModificationError{Err: ErrModICECandidatePoolSize}
	}
	return nil
}

func (r *RTCPeerConnection) setICEServers(config RTCConfiguration) error {
	panic("TODO")
	//if len(config.ICEServers) > 0 {
	//	var servers [][]ice.URL
	//	for _, server := range config.ICEServers {
	//		var urls []ice.URL
	//		for _, rawURL := range server.URLs {
	//			url, err := parseICEServer(server, rawURL)
	//			if err != nil {
	//				return err
	//			}
	//			urls = append(urls, url)
	//		}
	//		if len(urls) > 0 {
	//			servers = append(servers, urls)
	//		}
	//	}
	//	// if len(servers) > 0 {
	//	// 	r.iceAgent.SetServers(servers)
	//	// }
	//}
	// return nil
}

func parseICEServer(server RTCICEServer, rawURL string) (ice.URL, error) {
	iceurl, err := ice.NewURL(rawURL)
	if err != nil {
		return iceurl, &SyntaxError{Err: err}
	}

	if iceurl.Type == ice.ServerTypeTURN {
		if server.Username == "" {
			return iceurl, &InvalidAccessError{Err: ErrNoTurnCred}
		}

		switch t := server.Credential.(type) {
		case string:
			if t == "" {
				return iceurl, &InvalidAccessError{Err: ErrNoTurnCred}
			} else if server.CredentialType != RTCICECredentialTypePassword {
				return iceurl, &InvalidAccessError{Err: ErrTurnCred}
			}

		case RTCOAuthCredential:
			if server.CredentialType != RTCICECredentialTypeOauth {
				return iceurl, &InvalidAccessError{Err: ErrTurnCred}
			}

		default:
			return iceurl, &InvalidAccessError{Err: ErrTurnCred}

		}
	}
	return iceurl, nil
}
