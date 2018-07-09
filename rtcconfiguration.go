package webrtc

import (
	"fmt"
	"strings"
)

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

const (
	stunPrefix  = "stun:"
	stunsPrefix = "stuns:"
	turnPrefix  = "turn:"
	turnsPrefix = "turns:"
)

func (c RTCICEServer) serverType() RTCServerType {
	for _, url := range c.URLs {
		if strings.HasPrefix(url, stunPrefix) || strings.HasPrefix(url, stunsPrefix) {
			return RTCServerTypeSTUN
		}
		if strings.HasPrefix(url, turnPrefix) || strings.HasPrefix(url, turnsPrefix) {
			return RTCServerTypeTURN
		}
	}
	return RTCServerTypeUnknown
}

func protocolAndHost(url string) (string, string, error) {
	if strings.HasPrefix(url, stunPrefix) {
		return "udp", url[len(stunPrefix):], nil
	}
	if strings.HasPrefix(url, stunsPrefix) {
		return "tcp", url[len(stunsPrefix):], nil
	}
	// TODO TURN urls
	return "", "", fmt.Errorf("Unknown protocol in URL %q", url)
}

// RTCConfiguration contains RTCPeerConfiguration options
type RTCConfiguration struct {
	// ICEServers holds multiple RTCICEServer instances, each describing one server which may be used by the ICE agent;
	// these are typically STUN and/or TURN servers. If this isn't specified, the ICE agent may choose to use its own ICE servers;
	// otherwise, the connection attempt will be made with no STUN or TURN server available, which limits the connection to local peers.
	ICEServers []RTCICEServer
}
