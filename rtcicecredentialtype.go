package webrtc

// RTCIceCredentialType indicates the type of credentials used to connect to
// an ICE server.
type RTCIceCredentialType int

const (
	// RTCIceCredentialTypePassword describes username and pasword based
	// credentials as described in https://tools.ietf.org/html/rfc5389.
	RTCIceCredentialTypePassword RTCIceCredentialType = iota + 1

	// RTCIceCredentialTypeOauth describes token based credential as described
	// in https://tools.ietf.org/html/rfc7635.
	RTCIceCredentialTypeOauth
)

// NewRTCIceCredentialType defines a procedure for creating a new
// RTCIceCredentialType from a raw string naming the ice credential type.
func NewRTCIceCredentialType(raw string) RTCIceCredentialType {
	switch raw {
	case "password":
		return RTCIceCredentialTypePassword
	case "oauth":
		return RTCIceCredentialTypeOauth
	default:
		return RTCIceCredentialType(Unknown)
	}
}

func (t RTCIceCredentialType) String() string {
	switch t {
	case RTCIceCredentialTypePassword:
		return "password"
	case RTCIceCredentialTypeOauth:
		return "oauth"
	default:
		return ErrUnknownType.Error()
	}
}
