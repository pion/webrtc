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
	stableStr = "stable"
	haveLocalOfferStr = "have-local-offer"
	haveRemoteOfferStr = "have-remote-offer"
	haveLocalPranswerStr = "have-local-pranswer"
	haveRemotePranswerStr = "have-remote-pranswer"
	closeStr = "closed"
)

// NewRTCSignalingState defines a procedure for creating a new
// RTCSignalingState from a raw string naming the signaling state.
func NewRTCSignalingState(raw string) RTCSignalingState {
	switch raw {
	case stableStr:
		return RTCSignalingStateStable
	case haveLocalOfferStr:
		return RTCSignalingStateHaveLocalOffer
	case haveRemoteOfferStr:
		return RTCSignalingStateHaveRemoteOffer
	case haveLocalPranswerStr:
		return RTCSignalingStateHaveLocalPranswer
	case haveRemotePranswerStr:
		return RTCSignalingStateHaveRemotePranswer
	case closeStr:
		return RTCSignalingStateClosed
	default:
		return RTCSignalingState(Unknown)
	}
}

func (t RTCSignalingState) String() string {
	switch t {
	case RTCSignalingStateStable:
		return stableStr
	case RTCSignalingStateHaveLocalOffer:
		return haveLocalOfferStr
	case RTCSignalingStateHaveRemoteOffer:
		return haveRemoteOfferStr
	case RTCSignalingStateHaveLocalPranswer:
		return haveLocalPranswerStr
	case RTCSignalingStateHaveRemotePranswer:
		return haveRemotePranswerStr
	case RTCSignalingStateClosed:
		return closeStr
	default:
		return ErrUnknownType.Error()
	}
}
