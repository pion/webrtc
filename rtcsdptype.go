package webrtc

import (
	"encoding/json"
	"strings"
)

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
	rtcSdpTypeOfferStr    = "offer"
	rtcSdpTypePranswerStr = "pranswer"
	rtcSdpTypeAnswerStr   = "answer"
	rtcSdpTypeRollbackStr = "rollback"
)

func newRTCSdpType(raw string) RTCSdpType {
	switch raw {
	case rtcSdpTypeOfferStr:
		return RTCSdpTypeOffer
	case rtcSdpTypePranswerStr:
		return RTCSdpTypePranswer
	case rtcSdpTypeAnswerStr:
		return RTCSdpTypeAnswer
	case rtcSdpTypeRollbackStr:
		return RTCSdpTypeRollback
	default:
		return RTCSdpType(Unknown)
	}
}

func (t RTCSdpType) String() string {
	switch t {
	case RTCSdpTypeOffer:
		return rtcSdpTypeOfferStr
	case RTCSdpTypePranswer:
		return rtcSdpTypePranswerStr
	case RTCSdpTypeAnswer:
		return rtcSdpTypeAnswerStr
	case RTCSdpTypeRollback:
		return rtcSdpTypeRollbackStr
	default:
		return ErrUnknownType.Error()
	}
}

// MarshalJSON enables JSON marshaling of a RTCSdpType
func (t RTCSdpType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// UnmarshalJSON enables JSON unmarshaling of a RTCSdpType
func (t *RTCSdpType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	default:
		return ErrUnknownType
	case "offer":
		*t = RTCSdpTypeOffer
	case "pranswer":
		*t = RTCSdpTypePranswer
	case "answer":
		*t = RTCSdpTypeAnswer
	case "rollback":
		*t = RTCSdpTypeRollback
	}

	return nil
}
