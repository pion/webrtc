package webrtc

// RTCIceTransportPolicy defines the ICE candidate policy [JSEP] (section 3.5.3.) used to
// surface the permitted candidates
type RTCIceTransportPolicy int

const (
	// RTCIceTransportPolicyRelay indicates only media relay candidates such as candidates passing
	// through a TURN server are used
	RTCIceTransportPolicyRelay RTCIceTransportPolicy = iota + 1

	// RTCIceTransportPolicyAll indicates any type of candidate is used
	RTCIceTransportPolicyAll
)

func NewRTCIceTransportPolicy(raw string) (unknown RTCIceTransportPolicy) {
	switch raw {
	case "relay":
		return RTCIceTransportPolicyRelay
	case "all":
		return RTCIceTransportPolicyAll
	default:
		return unknown
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
