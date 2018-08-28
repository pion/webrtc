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

// This is done this way because of a linter.
const (
	offerStr = "offer"
	pranswerStr = "pranswer"
	answerStr = "answer"
	rollbackStr = "rollback"
)

// NewRTCSdpType defines a procedure for creating a new RTCSdpType from a raw
// string naming the session description protocol type.
func NewRTCSdpType(raw string) RTCSdpType {
	switch raw {
	case offerStr:
		return RTCSdpTypeOffer
	case pranswerStr:
		return RTCSdpTypePranswer
	case answerStr:
		return RTCSdpTypeAnswer
	case rollbackStr:
		return RTCSdpTypeRollback
	default:
		return RTCSdpType(Unknown)
	}
}

func (t RTCSdpType) String() string {
	switch t {
	case RTCSdpTypeOffer:
		return offerStr
	case RTCSdpTypePranswer:
		return pranswerStr
	case RTCSdpTypeAnswer:
		return answerStr
	case RTCSdpTypeRollback:
		return rollbackStr
	default:
		return ErrUnknownType.Error()
	}
}
