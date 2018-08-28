package webrtc

// RTCIceTransportPolicy defines the ICE candidate policy surface the
// permitted candidates. Only these candidates are used for connectivity checks.
type RTCIceTransportPolicy int

const (
	// RTCIceTransportPolicyRelay indicates only media relay candidates such
	// as candidates passing through a TURN server are used.
	RTCIceTransportPolicyRelay RTCIceTransportPolicy = iota + 1

	// RTCIceTransportPolicyAll indicates any type of candidate is used.
	RTCIceTransportPolicyAll
)

// NewRTCIceTransportPolicy defines a procedure for creating a new
// RTCIceTransportPolicy from a raw string naming the ice transport policy.
func NewRTCIceTransportPolicy(raw string) RTCIceTransportPolicy {
	switch raw {
	case "relay":
		return RTCIceTransportPolicyRelay
	case "all":
		return RTCIceTransportPolicyAll
	default:
		return RTCIceTransportPolicy(Unknown)
	}
}

func (t RTCIceTransportPolicy) String() string {
	switch t {
	case RTCIceTransportPolicyRelay:
		return "relay"
	case RTCIceTransportPolicyAll:
		return "all"
	default:
		return ErrUnknownType.Error()
	}
}
