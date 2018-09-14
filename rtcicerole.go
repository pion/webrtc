package webrtc

// RTCIceRole describes the role ice.Agent is playing in selecting the
// preferred the candidate pair.
type RTCIceRole int

const (
	// RTCIceRoleControlling indicates that the ICE agent that is responsible
	// for selecting the final choice of candidate pairs and signaling them
	// through STUN and an updated offer, if needed. In any session, one agent
	// is always controlling. The other is the controlled agent.
	RTCIceRoleControlling RTCIceRole = iota + 1

	// RTCIceRoleControlled indicates that an ICE agent that waits for the
	// controlling agent to select the final choice of candidate pairs.
	RTCIceRoleControlled
)

// This is done this way because of a linter.
const (
	rtcIceRoleControllingStr = "controlling"
	rtcIceRoleControlledStr  = "controlled"
)

func newRTCIceRole(raw string) RTCIceRole {
	switch raw {
	case rtcIceRoleControllingStr:
		return RTCIceRoleControlling
	case rtcIceRoleControlledStr:
		return RTCIceRoleControlled
	default:
		return RTCIceRole(Unknown)
	}
}

func (t RTCIceRole) String() string {
	switch t {
	case RTCIceRoleControlling:
		return rtcIceRoleControllingStr
	case RTCIceRoleControlled:
		return rtcIceRoleControlledStr
	default:
		return ErrUnknownType.Error()
	}
}
