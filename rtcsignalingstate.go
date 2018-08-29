package webrtc

// RTCSignalingState indicates the signaling state of the offer/answer process.
type RTCSignalingState int

const (
	// RTCSignalingStateStable indicates there is no offer/answer exchange in
	// progress. This is also the initial state, in which case the local and
	// remote descriptions are nil.
	RTCSignalingStateStable RTCSignalingState = iota + 1

	// RTCSignalingStateHaveLocalOffer indicates that a local description, of
	// type "offer", has been successfully applied.
	RTCSignalingStateHaveLocalOffer

	// RTCSignalingStateHaveRemoteOffer indicates that a remote description, of
	// type "offer", has been successfully applied.
	RTCSignalingStateHaveRemoteOffer

	// RTCSignalingStateHaveLocalPranswer indicates that a remote description
	// of type "offer" has been successfully applied and a local description
	// of type "pranswer" has been successfully applied.
	RTCSignalingStateHaveLocalPranswer

	// RTCSignalingStateHaveRemotePranswer indicates that a local description
	// of type "offer" has been successfully applied and a remote description
	// of type "pranswer" has been successfully applied.
	RTCSignalingStateHaveRemotePranswer

	// RTCSignalingStateClosed indicates The RTCPeerConnection has been closed.
	RTCSignalingStateClosed
)

// This is done this way because of a linter.
const (
	rtcSignalingStateStableStr             = "stable"
	rtcSignalingStateHaveLocalOfferStr     = "have-local-offer"
	rtcSignalingStateHaveRemoteOfferStr    = "have-remote-offer"
	rtcSignalingStateHaveLocalPranswerStr  = "have-local-pranswer"
	rtcSignalingStateHaveRemotePranswerStr = "have-remote-pranswer"
	rtcSignalingStateClosedStr             = "closed"
)

// NewRTCSignalingState defines a procedure for creating a new
// RTCSignalingState from a raw string naming the signaling state.
func NewRTCSignalingState(raw string) RTCSignalingState {
	switch raw {
	case rtcSignalingStateStableStr:
		return RTCSignalingStateStable
	case rtcSignalingStateHaveLocalOfferStr:
		return RTCSignalingStateHaveLocalOffer
	case rtcSignalingStateHaveRemoteOfferStr:
		return RTCSignalingStateHaveRemoteOffer
	case rtcSignalingStateHaveLocalPranswerStr:
		return RTCSignalingStateHaveLocalPranswer
	case rtcSignalingStateHaveRemotePranswerStr:
		return RTCSignalingStateHaveRemotePranswer
	case rtcSignalingStateClosedStr:
		return RTCSignalingStateClosed
	default:
		return RTCSignalingState(Unknown)
	}
}

func (t RTCSignalingState) String() string {
	switch t {
	case RTCSignalingStateStable:
		return rtcSignalingStateStableStr
	case RTCSignalingStateHaveLocalOffer:
		return rtcSignalingStateHaveLocalOfferStr
	case RTCSignalingStateHaveRemoteOffer:
		return rtcSignalingStateHaveRemoteOfferStr
	case RTCSignalingStateHaveLocalPranswer:
		return rtcSignalingStateHaveLocalPranswerStr
	case RTCSignalingStateHaveRemotePranswer:
		return rtcSignalingStateHaveRemotePranswerStr
	case RTCSignalingStateClosed:
		return rtcSignalingStateClosedStr
	default:
		return ErrUnknownType.Error()
	}
}
