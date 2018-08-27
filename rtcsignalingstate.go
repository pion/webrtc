package webrtc

// RTCSignalingState indicates the policy of the offer/answer process
type RTCSignalingState int

const (
	// RTCSignalingStateStable indicates there is no offer­answer exchange in progress.
	RTCSignalingStateStable RTCSignalingState = iota + 1

	// RTCSignalingStateHaveLocalOffer indicates A local description, of type "offer", has been successfully applied.
	RTCSignalingStateHaveLocalOffer

	// RTCSignalingStateHaveRemoteOffer indicates A remote description, of type "offer", has been successfully applied.
	RTCSignalingStateHaveRemoteOffer

	// RTCSignalingStateHaveLocalPranswer indicates A remote description of type "offer" has been successfully applied
	// and a local description of type "pranswer" has been successfully applied.
	RTCSignalingStateHaveLocalPranswer

	// RTCSignalingStateHaveRemotePranswer indicates A local description of type "offer" has been successfully applied
	// and a remote description of type "pranswer" has been successfully applied.
	RTCSignalingStateHaveRemotePranswer

	// RTCSignalingStateClosed indicates The RTCPeerConnection has been closed.
	RTCSignalingStateClosed
)

func NewRTCSignalingState(raw string) RTCSignalingState {
	switch raw {
	case "stable":
		return RTCSignalingStateStable
	case "have-local-offer":
		return RTCSignalingStateHaveLocalOffer
	case "have-remote-offer":
		return RTCSignalingStateHaveRemoteOffer
	case "have-local-pranswer":
		return RTCSignalingStateHaveLocalPranswer
	case "have-remote-pranswer":
		return RTCSignalingStateHaveRemotePranswer
	case "closed":
		return RTCSignalingStateClosed
	default:
		return RTCSignalingState(Unknown)
	}
}

func (t RTCSignalingState) String() string {
	switch t {
	case RTCSignalingStateStable:
		return "stable"
	case RTCSignalingStateHaveLocalOffer:
		return "have-local-offer"
	case RTCSignalingStateHaveRemoteOffer:
		return "have-remote-offer"
	case RTCSignalingStateHaveLocalPranswer:
		return "have-local-pranswer"
	case RTCSignalingStateHaveRemotePranswer:
		return "have-remote-pranswer"
	case RTCSignalingStateClosed:
		return "closed"
	default:
		return ErrUnknownType.Error()
	}
}
