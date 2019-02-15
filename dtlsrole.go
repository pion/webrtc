package webrtc

// DTLSRole indicates the role of the DTLS transport.
type DTLSRole byte

const (
	// DTLSRoleAuto defines the DLTS role is determined based on
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
