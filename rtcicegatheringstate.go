package webrtc

// RTCIceGatheringState describes the state of the candidate gathering process.
type RTCIceGatheringState int

const (
	// RTCIceGatheringStateNew indicates that any of the RTCIceTransports are
	// in the "new" gathering state and none of the transports are in the
	// "gathering" state, or there are no transports.
	RTCIceGatheringStateNew RTCIceGatheringState = iota + 1

	// RTCIceGatheringStateGathering indicates that any of the RTCIceTransports
	// are in the "gathering" state.
	RTCIceGatheringStateGathering

	// RTCIceGatheringStateComplete indicates that at least one RTCIceTransport
	// exists, and all RTCIceTransports are in the "completed" gathering state.
	RTCIceGatheringStateComplete
)

// This is done this way because of a linter.
const (
	rtcIceGatheringStateNewStr       = "new"
	rtcIceGatheringStateGatheringStr = "gathering"
	rtcIceGatheringStateCompleteStr  = "complete"
)

func newRTCIceGatheringState(raw string) RTCIceGatheringState {
	switch raw {
	case rtcIceGatheringStateNewStr:
		return RTCIceGatheringStateNew
	case rtcIceGatheringStateGatheringStr:
		return RTCIceGatheringStateGathering
	case rtcIceGatheringStateCompleteStr:
		return RTCIceGatheringStateComplete
	default:
		return RTCIceGatheringState(Unknown)
	}
}

func (t RTCIceGatheringState) String() string {
	switch t {
	case RTCIceGatheringStateNew:
		return rtcIceGatheringStateNewStr
	case RTCIceGatheringStateGathering:
		return rtcIceGatheringStateGatheringStr
	case RTCIceGatheringStateComplete:
		return rtcIceGatheringStateCompleteStr
	default:
		return ErrUnknownType.Error()
	}
}
