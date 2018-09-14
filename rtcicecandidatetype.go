package webrtc

// RTCIceCandidateType represents the type of the ICE candidate used.
type RTCIceCandidateType int

const (
	// RTCIceCandidateTypeHost indicates that the candidate is of Host type as
	// described in https://tools.ietf.org/html/rfc8445#section-5.1.1.1. A
	// candidate obtained by binding to a specific port from an IP address on
	// the host. This includes IP addresses on physical interfaces and logical
	// ones, such as ones obtained through VPNs.
	RTCIceCandidateTypeHost RTCIceCandidateType = iota + 1

	// RTCIceCandidateTypeSrflx indicates the the candidate is of Server
	// Reflexive type as described
	// https://tools.ietf.org/html/rfc8445#section-5.1.1.2. A candidate type
	// whose IP address and port are a binding allocated by a NAT for an ICE
	// agent after it sends a packet through the NAT to a server, such as a
	// STUN server.
	RTCIceCandidateTypeSrflx

	// RTCIceCandidateTypePrflx indicates that the candidate is of Peer
	// Reflexive type. A candidate type whose IP address and port are a binding
	// allocated by a NAT for an ICE agent after it sends a packet through the
	// NAT to its peer.
	RTCIceCandidateTypePrflx

	// RTCIceCandidateTypeRelay indicates the the candidate is of Relay type as
	// described in https://tools.ietf.org/html/rfc8445#section-5.1.1.2. A
	// candidate type obtained from a relay server, such as a TURN server.
	RTCIceCandidateTypeRelay
)

// This is done this way because of a linter.
const (
	rtcIceCandidateTypeHostStr  = "host"
	rtcIceCandidateTypeSrflxStr = "srflx"
	rtcIceCandidateTypePrflxStr = "prflx"
	rtcIceCandidateTypeRelayStr = "relay"
)

func newRTCIceCandidateType(raw string) RTCIceCandidateType {
	switch raw {
	case rtcIceCandidateTypeHostStr:
		return RTCIceCandidateTypeHost
	case rtcIceCandidateTypeSrflxStr:
		return RTCIceCandidateTypeSrflx
	case rtcIceCandidateTypePrflxStr:
		return RTCIceCandidateTypePrflx
	case rtcIceCandidateTypeRelayStr:
		return RTCIceCandidateTypeRelay
	default:
		return RTCIceCandidateType(Unknown)
	}
}

func (t RTCIceCandidateType) String() string {
	switch t {
	case RTCIceCandidateTypeHost:
		return rtcIceCandidateTypeHostStr
	case RTCIceCandidateTypeSrflx:
		return rtcIceCandidateTypeSrflxStr
	case RTCIceCandidateTypePrflx:
		return rtcIceCandidateTypePrflxStr
	case RTCIceCandidateTypeRelay:
		return rtcIceCandidateTypeRelayStr
	default:
		return ErrUnknownType.Error()
	}
}
