package webrtc

// ICETransportPolicy defines the ICE candidate policy surface the
// permitted candidates. Only these candidates are used for connectivity checks.
type ICETransportPolicy int

const (
	// ICETransportPolicyRelay indicates only media relay candidates such
	// as candidates passing through a TURN server are used.
	ICETransportPolicyRelay ICETransportPolicy = iota + 1

	// ICETransportPolicyAll indicates any type of candidate is used.
	ICETransportPolicyAll
)

// This is done this way because of a linter.
const (
	iceTransportPolicyRelayStr = "relay"
	iceTransportPolicyAllStr   = "all"
)

func newICETransportPolicy(raw string) ICETransportPolicy {
	switch raw {
	case iceTransportPolicyRelayStr:
		return ICETransportPolicyRelay
	case iceTransportPolicyAllStr:
		return ICETransportPolicyAll
	default:
		return ICETransportPolicy(Unknown)
	}
}

func (t ICETransportPolicy) String() string {
	switch t {
	case ICETransportPolicyRelay:
		return iceTransportPolicyRelayStr
	case ICETransportPolicyAll:
		return iceTransportPolicyAllStr
	default:
		return ErrUnknownType.Error()
	}
}
