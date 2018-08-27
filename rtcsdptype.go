package webrtc

// RTCSdpType describes the type of an RTCSessionDescription
type RTCSdpType int

const (
	// RTCSdpTypeOffer indicates that a description MUST be treated as an SDP offer.
	RTCSdpTypeOffer RTCSdpType = iota + 1

	// RTCSdpTypePranswer indicates that a description MUST be treated as an SDP answer, but not a final answer.
	RTCSdpTypePranswer

	// RTCSdpTypeAnswer indicates that a description MUST be treated as an SDP final answer, and the offer-answer
	// exchange MUST be considered complete.
	RTCSdpTypeAnswer

	// RTCSdpTypeRollback indicates that a description MUST be treated as canceling the current SDP negotiation
	// and moving the SDP offer and answer back to what it was in the previous stable state.
	RTCSdpTypeRollback
)

func NewRTCSdpType(raw string) (unknown RTCSdpType) {
	switch raw {
	case "offer":
		return RTCSdpTypeOffer
	case "pranswer":
		return RTCSdpTypePranswer
	case "answer":
		return RTCSdpTypeAnswer
	case "rollback":
		return RTCSdpTypeRollback
	default:
		return unknown
	}
}

func (t RTCSdpType) String() string {
	switch t {
	case RTCSdpTypeOffer:
		return "offer"
	case RTCSdpTypePranswer:
		return "pranswer"
	case RTCSdpTypeAnswer:
		return "answer"
	case RTCSdpTypeRollback:
		return "rollback"
	default:
		return ErrUnknownType.Error()
	}
}
