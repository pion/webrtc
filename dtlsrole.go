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

const (
	defaultDtlsRoleAnswer = DTLSRoleServer
	defaultDtlsRoleOffer  = DTLSRoleAuto
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
// role can been determined from it. The decision is made from the first role we we parse.
// If no role can be found we return DTLSRoleAuto
func dtlsRoleFromRemoteSDP(sessionDescription *sdp.SessionDescription) DTLSRole {
	if sessionDescription == nil {
		return DTLSRoleAuto
	}

	for _, mediaSection := range sessionDescription.MediaDescriptions {
		for _, attribute := range mediaSection.Attributes {
			if attribute.Key == "setup" {
				switch attribute.Value {
				case sdp.ConnectionRoleActive.String():
					return DTLSRoleClient
				case sdp.ConnectionRolePassive.String():
					return DTLSRoleServer
				default:
					return DTLSRoleAuto
				}
			}
		}
	}

	return DTLSRoleAuto
}

func connectionRoleFromDtlsRole(d DTLSRole) sdp.ConnectionRole {
	switch d {
	case DTLSRoleClient:
		return sdp.ConnectionRoleActive
	case DTLSRoleServer:
		return sdp.ConnectionRolePassive
	case DTLSRoleAuto:
		return sdp.ConnectionRoleActpass
	default:
		return sdp.ConnectionRole(0)
	}
}
