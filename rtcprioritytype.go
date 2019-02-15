package webrtc

// PriorityType determines the priority type of a data channel.
type PriorityType int

const (
	// PriorityTypeVeryLow corresponds to "below normal".
	PriorityTypeVeryLow PriorityType = iota + 1

	// PriorityTypeLow corresponds to "normal".
	PriorityTypeLow

	// PriorityTypeMedium corresponds to "high".
	PriorityTypeMedium

	// PriorityTypeHigh corresponds to "extra high".
	PriorityTypeHigh
)

// This is done this way because of a linter.
const (
	priorityTypeVeryLowStr = "very-low"
	priorityTypeLowStr     = "low"
	priorityTypeMediumStr  = "medium"
	priorityTypeHighStr    = "high"
)

func newPriorityTypeFromString(raw string) PriorityType {
	switch raw {
	case priorityTypeVeryLowStr:
		return PriorityTypeVeryLow
	case priorityTypeLowStr:
		return PriorityTypeLow
	case priorityTypeMediumStr:
		return PriorityTypeMedium
	case priorityTypeHighStr:
		return PriorityTypeHigh
	default:
		return PriorityType(Unknown)
	}
}

func newPriorityTypeFromUint16(raw uint16) PriorityType {
	switch {
	case raw <= 128:
		return PriorityTypeVeryLow
	case 129 <= raw && raw <= 256:
		return PriorityTypeLow
	case 257 <= raw && raw <= 512:
		return PriorityTypeMedium
	case 513 <= raw:
		return PriorityTypeHigh
	default:
		return PriorityType(Unknown)
	}
}

func (p PriorityType) String() string {
	switch p {
	case PriorityTypeVeryLow:
		return priorityTypeVeryLowStr
	case PriorityTypeLow:
		return priorityTypeLowStr
	case PriorityTypeMedium:
		return priorityTypeMediumStr
	case PriorityTypeHigh:
		return priorityTypeHighStr
	default:
		return ErrUnknownType.Error()
	}
}
