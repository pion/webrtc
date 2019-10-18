package webrtc

import (
	"github.com/pion/sdp/v2"
)

// DTLSRole indicates the role of the DTLS transport.
type DTLSRole byte

const (
	// DTLSRoleAuto defines the DTLS role is determined based on
	// the resolved ICE role: the ICE controlled role acts as the DTLS
	// client and the ICE controlling role acts as the DTLS server.
	DTLSRoleAuto DTLSRole = iota + 1

	// DTLSRoleClient defines the DTLS client role.
	DTLSRoleClient

	// DTLSRoleServer defines the DTLS server role.
	DTLSRoleServer
)

func (r DTLSRole) String() string {
	switch r {
	case DTLSRoleAuto:
		return "auto"
	case DTLSRoleClient:
		return "client"
	case DTLSRoleServer:
		return "server"
	default:
		return unknownStr
	}
}

// Iterate a SessionDescription from a remote to determine if an explicit
// role can been determined for local connection. The decision is made from the first role we we parse.
// If no role can be found we return DTLSRoleAuto
func dtlsRoleFromRemoteSDP(sessionDescription *sdp.SessionDescription) DTLSRole {
	if sessionDescription == nil {
		return DTLSRoleAuto
	}

	for _, mediaSection := range sessionDescription.MediaDescriptions {
		for _, attribute := range mediaSection.Attributes {
			if attribute.Key == "setup" {
				switch attribute.Value {
				case "active":
					return DTLSRoleServer
				case "passive":
					return DTLSRoleClient
				default:
					return DTLSRoleAuto
				}
			}
		}
	}

	return DTLSRoleAuto
}
