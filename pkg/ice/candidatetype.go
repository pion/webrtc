package ice

// CandidateType represents the type of candidate
type CandidateType byte

// CandidateType enum
const (
	CandidateTypeHost CandidateType = iota + 1
	CandidateTypeServerReflexive
	// CandidateTypePeerReflexive // TODO
	// CandidateTypeRelay // TODO
)

// String makes CandidateType printable
func (c CandidateType) String() string {
	switch c {
	case CandidateTypeHost:
		return "host"
	case CandidateTypeServerReflexive:
		return "srflx"
		// case CandidateTypePeerReflexive:
		// 	return "prflx"
		// case CandidateTypeRelay:
		// 	return "relay"
	}
	return "Unknown candidate type"
}

// Preference returns the preference weight of a CandidateType
func (c CandidateType) Preference() uint16 {
	switch c {
	case CandidateTypeHost:
		return 126
	case CandidateTypeServerReflexive:
		return 100
	}
	return 0
}
