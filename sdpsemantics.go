package webrtc

// SDPSemantics determines which style of SDP offers and answers
// can be used
type SDPSemantics int

const (
	// SDPSemanticsUnifiedPlan uses unified-plan offers and answers
	// (the default in Chrome since M72)
	// https://tools.ietf.org/html/draft-roach-mmusic-unified-plan-00
	SDPSemanticsUnifiedPlan SDPSemantics = iota

	// SDPSemanticsPlanB uses plan-b offers and answers
	// NB: This format should be considered deprecated
	// https://tools.ietf.org/html/draft-uberti-rtcweb-plan-00
	SDPSemanticsPlanB

	// SDPSemanticsUnifiedPlanWithFallback prefers unified-plan
	// offers and answers, but will respond to a plan-b offer
	// with a plan-b answer
	SDPSemanticsUnifiedPlanWithFallback
)

const (
	sdpSemanticsUnifiedPlanWithFallback = "unified-plan-with-fallback"
	sdpSemanticsUnifiedPlan             = "unified-plan"
	sdpSemanticsPlanB                   = "plan-b"
)

func (s SDPSemantics) String() string {
	switch s {
	case SDPSemanticsUnifiedPlanWithFallback:
		return sdpSemanticsUnifiedPlanWithFallback
	case SDPSemanticsUnifiedPlan:
		return sdpSemanticsUnifiedPlan
	case SDPSemanticsPlanB:
		return sdpSemanticsPlanB
	default:
		return ErrUnknownType.Error()
	}
}
