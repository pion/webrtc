package ice

// Component describes if the ice transport is used for RTP
// (or RTCP multiplexing).
type Component int

const (
	// ComponentRTP indicates that the ICE Transport is used for RTP (or
	// RTCP multiplexing), as defined in
	// https://tools.ietf.org/html/rfc5245#section-4.1.1.1. Protocols
	// multiplexed with RTP (e.g. data channel) share its component ID. This
	// represents the component-id value 1 when encoded in candidate-attribute.
	ComponentRTP Component = iota + 1

	// ComponentRTCP indicates that the ICE Transport is used for RTCP as
	// defined by https://tools.ietf.org/html/rfc5245#section-4.1.1.1. This
	// represents the component-id value 2 when encoded in candidate-attribute.
	ComponentRTCP
)

// This is done this way because of a linter.
const (
	componentRTPStr  = "rtp"
	componentRTCPStr = "rtcp"
)

func newComponent(raw string) Component {
	switch raw {
	case componentRTPStr:
		return ComponentRTP
	case componentRTCPStr:
		return ComponentRTCP
	default:
		return Component(Unknown)
	}
}

func (t Component) String() string {
	switch t {
	case ComponentRTP:
		return componentRTPStr
	case ComponentRTCP:
		return componentRTCPStr
	default:
		return ErrUnknownType.Error()
	}
}
