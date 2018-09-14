package webrtc

// RTCPriorityType determines the priority type of a data channel.
type RTCPriorityType int

const (
	// RTCPriorityTypeVeryLow corresponds to "below normal".
	RTCPriorityTypeVeryLow RTCPriorityType = iota + 1

	// RTCPriorityTypeLow corresponds to "normal".
	RTCPriorityTypeLow

	// RTCPriorityTypeMedium corresponds to "high".
	RTCPriorityTypeMedium

	// RTCPriorityTypeHigh corresponds to "extra high".
	RTCPriorityTypeHigh
)

// This is done this way because of a linter.
const (
	rtcPriorityTypeVeryLowStr = "very-low"
	rtcPriorityTypeLowStr     = "low"
	rtcPriorityTypeMediumStr  = "medium"
	rtcPriorityTypeHighStr    = "high"
)

func newRTCPriorityTypeFromString(raw string) RTCPriorityType {
	switch raw {
	case rtcPriorityTypeVeryLowStr:
		return RTCPriorityTypeVeryLow
	case rtcPriorityTypeLowStr:
		return RTCPriorityTypeLow
	case rtcPriorityTypeMediumStr:
		return RTCPriorityTypeMedium
	case rtcPriorityTypeHighStr:
		return RTCPriorityTypeHigh
	default:
		return RTCPriorityType(Unknown)
	}
}

func newRTCPriorityTypeFromUint16(raw uint16) RTCPriorityType {
	switch {
	case raw <= 128:
		return RTCPriorityTypeVeryLow
	case 129 <= raw && raw <= 256:
		return RTCPriorityTypeLow
	case 257 <= raw && raw <= 512:
		return RTCPriorityTypeMedium
	case 513 <= raw:
		return RTCPriorityTypeHigh
	default:
		return RTCPriorityType(Unknown)
	}
}

func (p RTCPriorityType) String() string {
	switch p {
	case RTCPriorityTypeVeryLow:
		return rtcPriorityTypeVeryLowStr
	case RTCPriorityTypeLow:
		return rtcPriorityTypeLowStr
	case RTCPriorityTypeMedium:
		return rtcPriorityTypeMediumStr
	case RTCPriorityTypeHigh:
		return rtcPriorityTypeHighStr
	default:
		return ErrUnknownType.Error()
	}
}
