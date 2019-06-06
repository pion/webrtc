package ice

// GatheringState describes the state of the candidate gathering process.
type GatheringState int

const (
	// GatheringStateNew indicates that any of the ICETransports are
	// in the "new" gathering state and none of the transports are in the
	// "gathering" state, or there are no transports.
	GatheringStateNew GatheringState = iota + 1

	// GatheringStateGathering indicates that any of the ICETransports
	// are in the "gathering" state.
	GatheringStateGathering

	// GatheringStateComplete indicates that at least one Transport
	// exists, and all ICETransports are in the "completed" gathering state.
	GatheringStateComplete
)

// This is done this way because of a linter.
const (
	gatheringStateNewStr       = "new"
	gatheringStateGatheringStr = "gathering"
	gatheringStateCompleteStr  = "complete"
)

// NewGatheringState takes a string and converts it to GatheringState
func NewGatheringState(raw string) GatheringState {
	switch raw {
	case gatheringStateNewStr:
		return GatheringStateNew
	case gatheringStateGatheringStr:
		return GatheringStateGathering
	case gatheringStateCompleteStr:
		return GatheringStateComplete
	default:
		return GatheringState(Unknown)
	}
}

func (t GatheringState) String() string {
	switch t {
	case GatheringStateNew:
		return gatheringStateNewStr
	case GatheringStateGathering:
		return gatheringStateGatheringStr
	case GatheringStateComplete:
		return gatheringStateCompleteStr
	default:
		return ErrUnknownType.Error()
	}
}
