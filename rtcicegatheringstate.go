package webrtc

// GatheringState describes the state of the candidate gathering process
type RTCIceGatheringState int

const (
	// GatheringStateNew indicates candidate gatering is not yet started
	RTCIceGatheringStateNew RTCIceGatheringState = iota + 1

	// GatheringStateGathering indicates candidate gatering is ongoing
	RTCIceGatheringStateGathering

	// GatheringStateComplete indicates candidate gatering has been completed
	RTCIceGatheringStateComplete
)

// This is done this way because of a linter.
const (
	rtcIceGatheringStateNewStr       = "new"
	rtcIceGatheringStateGatheringStr = "gathering"
	rtcIceGatheringStateCompleteStr  = "complete"
)

// NewRTCIceGatheringState defines a procedure for creating a new
// RTCIceGatheringState from a raw string naming the gathering state.
func NewRTCIceGatheringState(raw string) RTCIceGatheringState {
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
