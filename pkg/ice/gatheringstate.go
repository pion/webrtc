package ice

// GatheringState describes the state of the candidate gathering process
type GatheringState int

const (
	// GatheringStateNew indicates candidate gatering is not yet started
	GatheringStateNew GatheringState = iota + 1

	// GatheringStateGathering indicates candidate gatering is ongoing
	GatheringStateGathering

	// GatheringStateComplete indicates candidate gatering has been completed
	GatheringStateComplete
)

func (t GatheringState) String() string {
	switch t {
	case GatheringStateNew:
		return "new"
	case GatheringStateGathering:
		return "gathering"
	case GatheringStateComplete:
		return "complete"
	default:
		return ErrUnknownType.Error()
	}
}
