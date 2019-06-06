package ice

// TransportPolicy defines the ICE candidate policy surface the
// permitted candidates. Only these candidates are used for connectivity checks.
type TransportPolicy int

// GatherPolicy is the ORTC equivalent of TransportPolicy
type GatherPolicy = TransportPolicy

const (
	// TransportPolicyAll indicates any type of candidate is used.
	TransportPolicyAll TransportPolicy = iota

	// TransportPolicyRelay indicates only media relay candidates such
	// as candidates passing through a TURN server are used.
	TransportPolicyRelay
)

// This is done this way because of a linter.
const (
	transportPolicyRelayStr = "relay"
	transportPolicyAllStr   = "all"
)

// NewTransportPolicy takes a string and converts it to TransportPolicy
func NewTransportPolicy(raw string) TransportPolicy {
	switch raw {
	case transportPolicyRelayStr:
		return TransportPolicyRelay
	case transportPolicyAllStr:
		return TransportPolicyAll
	default:
		return TransportPolicy(Unknown)
	}
}

func (t TransportPolicy) String() string {
	switch t {
	case TransportPolicyRelay:
		return transportPolicyRelayStr
	case TransportPolicyAll:
		return transportPolicyAllStr
	default:
		return ErrUnknownType.Error()
	}
}
