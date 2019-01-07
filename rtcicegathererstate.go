package webrtc

// RTCIceGathererState represents the current state of the ICE gatherer.
type RTCIceGathererState byte

const (
	// RTCIceGathererStateNew indicates object has been created but
	// gather() has not been called.
	RTCIceGathererStateNew RTCIceGathererState = iota + 1

	// RTCIceGathererStateGathering indicates gather() has been called,
	// and the RTCIceGatherer is in the process of gathering candidates.
	RTCIceGathererStateGathering

	// RTCIceGathererStateComplete indicates the RTCIceGatherer has completed gathering.
	RTCIceGathererStateComplete

	// RTCIceGathererStateClosed indicates the closed state can only be entered
	// when the RTCIceGatherer has been closed intentionally by calling close().
	RTCIceGathererStateClosed
)

func (s RTCIceGathererState) String() string {
	switch s {
	case RTCIceGathererStateNew:
		return "new"
	case RTCIceGathererStateGathering:
		return "gathering"
	case RTCIceGathererStateComplete:
		return "complete"
	case RTCIceGathererStateClosed:
		return "closed"
	default:
		return unknownStr
	}
}
