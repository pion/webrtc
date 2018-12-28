package webrtc

// RTCQuicRole indicates the role of the Quic transport.
type RTCQuicRole byte

const (
	// RTCQuicRoleAuto defines the Quic role is determined based on
	// the resolved ICE role: the ICE controlled role acts as the Quic
	// client and the ICE controlling role acts as the Quic server.
	RTCQuicRoleAuto RTCQuicRole = iota + 1

	// RTCQuicRoleClient defines the Quic client role.
	RTCQuicRoleClient

	// RTCQuicRoleServer defines the Quic server role.
	RTCQuicRoleServer
)

func (r RTCQuicRole) String() string {
	switch r {
	case RTCQuicRoleAuto:
		return "auto"
	case RTCQuicRoleClient:
		return "client"
	case RTCQuicRoleServer:
		return "server"
	default:
		return "Unknown Quic role"
	}
}
