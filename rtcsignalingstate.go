package webrtc

import (
	"fmt"

	"github.com/pions/webrtc/pkg/rtcerr"
	"github.com/pkg/errors"
)

type rtcStateChangeOp int

const (
	rtcStateChangeOpSetLocal rtcStateChangeOp = iota + 1
	rtcStateChangeOpSetRemote
)

func (op rtcStateChangeOp) String() string {
	switch op {
	case rtcStateChangeOpSetLocal:
		return "SetLocal"
	case rtcStateChangeOpSetRemote:
		return "SetRemote"
	default:
		return "Unknown State Change Operation"
	}
}

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

func newRTCSignalingState(raw string) RTCSignalingState {
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

func checkNextSignalingState(cur, next RTCSignalingState, op rtcStateChangeOp, sdpType RTCSdpType) (RTCSignalingState, error) {
	// Special case for rollbacks
	if sdpType == RTCSdpTypeRollback && cur == RTCSignalingStateStable {
		return cur, &rtcerr.InvalidModificationError{
			Err: errors.New("Can't rollback from stable state"),
		}
	}

	// 4.3.1 valid state transitions
	switch cur {
	case RTCSignalingStateStable:
		switch op {
		case rtcStateChangeOpSetLocal:
			// stable->SetLocal(offer)->have-local-offer
			if sdpType == RTCSdpTypeOffer && next == RTCSignalingStateHaveLocalOffer {
				return next, nil
			}
		case rtcStateChangeOpSetRemote:
			// stable->SetRemote(offer)->have-remote-offer
			if sdpType == RTCSdpTypeOffer && next == RTCSignalingStateHaveRemoteOffer {
				return next, nil
			}
		}
	case RTCSignalingStateHaveLocalOffer:
		if op == rtcStateChangeOpSetRemote {
			switch sdpType {
			// have-local-offer->SetRemote(answer)->stable
			case RTCSdpTypeAnswer:
				if next == RTCSignalingStateStable {
					return next, nil
				}
			// have-local-offer->SetRemote(pranswer)->have-remote-pranswer
			case RTCSdpTypePranswer:
				if next == RTCSignalingStateHaveRemotePranswer {
					return next, nil
				}
			}
		}
	case RTCSignalingStateHaveRemotePranswer:
		if op == rtcStateChangeOpSetRemote && sdpType == RTCSdpTypeAnswer {
			// have-remote-pranswer->SetRemote(answer)->stable
			if next == RTCSignalingStateStable {
				return next, nil
			}
		}
	case RTCSignalingStateHaveRemoteOffer:
		if op == rtcStateChangeOpSetLocal {
			switch sdpType {
			// have-remote-offer->SetLocal(answer)->stable
			case RTCSdpTypeAnswer:
				if next == RTCSignalingStateStable {
					return next, nil
				}
			// have-remote-offer->SetLocal(pranswer)->have-local-pranswer
			case RTCSdpTypePranswer:
				if next == RTCSignalingStateHaveLocalPranswer {
					return next, nil
				}
			}
		}
	case RTCSignalingStateHaveLocalPranswer:
		if op == rtcStateChangeOpSetLocal && sdpType == RTCSdpTypeAnswer {
			// have-local-pranswer->SetLocal(answer)->stable
			if next == RTCSignalingStateStable {
				return next, nil
			}
		}
	}

	return cur, &rtcerr.InvalidModificationError{
		Err: fmt.Errorf("Invalid proposed signaling state transition %s->%s(%s)->%s", cur, op, sdpType, next),
	}
}
