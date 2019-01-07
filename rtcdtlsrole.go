package webrtc

// RTCDtlsRole indicates the role of the DTLS transport.
type RTCDtlsRole byte

const (
	// RTCDtlsRoleAuto defines the DLTS role is determined based on
	// the resolved ICE role: the ICE controlled role acts as the DTLS
	// client and the ICE controlling role acts as the DTLS server.
	RTCDtlsRoleAuto RTCDtlsRole = iota + 1

	// RTCDtlsRoleClient defines the DTLS client role.
	RTCDtlsRoleClient

	// RTCDtlsRoleServer defines the DTLS server role.
	RTCDtlsRoleServer
)

func (r RTCDtlsRole) String() string {
	switch r {
	case RTCDtlsRoleAuto:
		return "auto"
	case RTCDtlsRoleClient:
		return "client"
	case RTCDtlsRoleServer:
		return "server"
	default:
		return unknownStr
	}
}
