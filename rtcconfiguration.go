package webrtc

import "strings"

// RTCCredentialType specifies the type of credentials provided
type RTCCredentialType string

var (
	// RTCCredentialTypePassword describes username+pasword based credentials
	RTCCredentialTypePassword RTCCredentialType = "password"
	// RTCCredentialTypeToken describes token based credentials
	RTCCredentialTypeToken RTCCredentialType = "token"
)

// RTCServerType is used to identify different ICE server types
type RTCServerType string

var (
	// RTCServerTypeSTUN is used to identify STUN servers. Prefix is stun:
	RTCServerTypeSTUN RTCServerType = "STUN"
	// RTCServerTypeTURN is used to identify TURN servers. Prefix is turn:
	RTCServerTypeTURN RTCServerType = "TURN"
	// RTCServerTypeUnknown is used when an ICE server can not be identified properly.
	RTCServerTypeUnknown RTCServerType = "UnknownType"
)

// RTCICEServer describes a single ICE server, as well as required credentials
type RTCICEServer struct {
	CredentialType RTCCredentialType
	URLs           []string
	Username       string
	Credential     string
}

func (c RTCICEServer) serverType() RTCServerType {
	for _, url := range c.URLs {
		if strings.HasPrefix(url, "stun:") {
			return RTCServerTypeSTUN
		}
		if strings.HasPrefix(url, "turn:") {
			return RTCServerTypeTURN
		}
	}
	return RTCServerTypeUnknown
}

// RTCConfiguration contains RTCPeerConfiguration options
type RTCConfiguration struct {
	ICEServers []RTCICEServer // An array of RTCIceServer objects, each describing one server which may be used by the ICE agent; these are typically STUN and/or TURN servers. If this isn't specified, the ICE agent may choose to use its own ICE servers; otherwise, the connection attempt will be made with no STUN or TURN server available, which limits the connection to local peers.
}
