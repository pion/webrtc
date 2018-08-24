package webrtc

// RTCIceCredentialType indicates the type of credentials used to connect to an ICE server
type RTCIceCredentialType int

const (
	// RTCIceCredentialTypePassword describes username+pasword based credentials
	RTCIceCredentialTypePassword RTCIceCredentialType = iota + 1
	// RTCIceCredentialTypeOauth describes token based credentials
	RTCIceCredentialTypeOauth
)

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
