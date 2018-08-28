package webrtc

// RTCSdpType describes the type of an RTCSessionDescription.
type RTCSdpType int

const (
	// RTCSdpTypeOffer indicates that a description MUST be treated as an SDP
	// offer.
	RTCSdpTypeOffer RTCSdpType = iota + 1

	// RTCSdpTypePranswer indicates that a description MUST be treated as an
	// SDP answer, but not a final answer. A description used as an SDP
	// pranswer may be applied as a response to an SDP offer, or an update to
	// a previously sent SDP pranswer.
	RTCSdpTypePranswer

	// RTCSdpTypeAnswer indicates that a description MUST be treated as an SDP
	// final answer, and the offer-answer exchange MUST be considered complete.
	// A description used as an SDP answer may be applied as a response to an
	// SDP offer or as an update to a previously sent SDP pranswer.
	RTCSdpTypeAnswer

	// RTCSdpTypeRollback indicates that a description MUST be treated as
	// canceling the current SDP negotiation and moving the SDP offer and
	// answer back to what it was in the previous stable state. Note the
	// local or remote SDP descriptions in the previous stable state could be
	// null if there has not yet been a successful offer-answer negotiation.
	RTCSdpTypeRollback
)

// NewRTCSdpType defines a proceedure for creating a new RTCSdpType from a raw
// string naming the session description protocol type.
func NewRTCSdpType(raw string) RTCSdpType {
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
		return RTCSdpType(Unknown)
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
