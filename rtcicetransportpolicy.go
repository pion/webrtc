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

// This is done this way because of a linter.
const (
	rtcIceTransportPolicyRelayStr = "relay"
	rtcIceTransportPolicyAllStr   = "all"
)

func newRTCIceTransportPolicy(raw string) RTCIceTransportPolicy {
	switch raw {
	case rtcIceTransportPolicyRelayStr:
		return RTCIceTransportPolicyRelay
	case rtcIceTransportPolicyAllStr:
		return RTCIceTransportPolicyAll
	default:
		return RTCIceTransportPolicy(Unknown)
	}
}

func (t RTCIceTransportPolicy) String() string {
	switch t {
	case RTCIceTransportPolicyRelay:
		return rtcIceTransportPolicyRelayStr
	case RTCIceTransportPolicyAll:
		return rtcIceTransportPolicyAllStr
	default:
		return ErrUnknownType.Error()
	}
}
