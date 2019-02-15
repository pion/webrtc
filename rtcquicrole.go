package webrtc

// QUICRole indicates the role of the Quic transport.
type QUICRole byte

const (
	// QUICRoleAuto defines the Quic role is determined based on
	// the resolved ICE role: the ICE controlled role acts as the Quic
	// client and the ICE controlling role acts as the Quic server.
	QUICRoleAuto QUICRole = iota + 1

	// QUICRoleClient defines the Quic client role.
	QUICRoleClient

	// QUICRoleServer defines the Quic server role.
	QUICRoleServer
)

func (r QUICRole) String() string {
	switch r {
	case QUICRoleAuto:
		return "auto"
	case QUICRoleClient:
		return "client"
	case QUICRoleServer:
		return "server"
	default:
		return unknownStr
	}
}
