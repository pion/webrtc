package ice

// Role describes the role ice.Agent is playing in selecting the
// preferred the candidate pair.
type Role int

const (
	// RoleControlling indicates that the ICE agent that is responsible
	// for selecting the final choice of candidate pairs and signaling them
	// through STUN and an updated offer, if needed. In any session, one agent
	// is always controlling. The other is the controlled agent.
	RoleControlling Role = iota + 1

	// RoleControlled indicates that an ICE agent that waits for the
	// controlling agent to select the final choice of candidate pairs.
	RoleControlled
)

// This is done this way because of a linter.
const (
	roleControllingStr = "controlling"
	roleControlledStr  = "controlled"
)

func newRole(raw string) Role {
	switch raw {
	case roleControllingStr:
		return RoleControlling
	case roleControlledStr:
		return RoleControlled
	default:
		return Role(Unknown)
	}
}

func (t Role) String() string {
	switch t {
	case RoleControlling:
		return roleControllingStr
	case RoleControlled:
		return roleControlledStr
	default:
		return ErrUnknownType.Error()
	}
}
