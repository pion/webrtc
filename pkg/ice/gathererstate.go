package ice

// GathererState represents the current state of the ICE gatherer.
type GathererState byte

const (
	// GathererStateNew indicates object has been created but
	// gather() has not been called.
	GathererStateNew GathererState = iota + 1

	// GathererStateGathering indicates gather() has been called,
	// and the Gatherer is in the process of gathering candidates.
	GathererStateGathering

	// GathererStateComplete indicates the Gatherer has completed gathering.
	GathererStateComplete

	// GathererStateClosed indicates the closed state can only be entered
	// when the Gatherer has been closed intentionally by calling close().
	GathererStateClosed
)

func (s GathererState) String() string {
	switch s {
	case GathererStateNew:
		return "new"
	case GathererStateGathering:
		return "gathering"
	case GathererStateComplete:
		return "complete"
	case GathererStateClosed:
		return "closed"
	default:
		return unknownStr
	}
}
