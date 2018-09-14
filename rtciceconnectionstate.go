package webrtc

// RTCIceConnectionState indicates signaling state of the Ice Connection.
type RTCIceConnectionState int

const (
	// RTCIceConnectionStateNew indicates that any of the RTCIceTransports are
	// in the "new" state and none of them are in the "checking", "disconnected"
	// or "failed" state, or all RTCIceTransports are in the "closed" state, or
	// there are no transports.
	RTCIceConnectionStateNew RTCIceConnectionState = iota + 1

	// RTCIceConnectionStateChecking indicates that any of the RTCIceTransports
	// are in the "checking" state and none of them are in the "disconnected"
	// or "failed" state.
	RTCIceConnectionStateChecking

	// RTCIceConnectionStateConnected indicates that all RTCIceTransports are
	// in the "connected", "completed" or "closed" state and at least one of
	// them is in the "connected" state.
	RTCIceConnectionStateConnected

	// RTCIceConnectionStateCompleted indicates that all RTCIceTransports are
	// in the "completed" or "closed" state and at least one of them is in the
	// "completed" state.
	RTCIceConnectionStateCompleted

	// RTCIceConnectionStateDisconnected indicates that any of the
	// RTCIceTransports are in the "disconnected" state and none of them are
	// in the "failed" state.
	RTCIceConnectionStateDisconnected

	// RTCIceConnectionStateFailed indicates that any of the RTCIceTransports
	// are in the "failed" state.
	RTCIceConnectionStateFailed

	// RTCIceConnectionStateClosed indicates that the RTCPeerConnection's
	// isClosed is true.
	RTCIceConnectionStateClosed
)

// This is done this way because of a linter.
const (
	rtcIceConnectionStateNewStr          = "new"
	rtcIceConnectionStateCheckingStr     = "checking"
	rtcIceConnectionStateConnectedStr    = "connected"
	rtcIceConnectionStateCompletedStr    = "completed"
	rtcIceConnectionStateDisconnectedStr = "disconnected"
	rtcIceConnectionStateFailedStr       = "failed"
	rtcIceConnectionStateClosedStr       = "closed"
)

func newRTCIceConnectionState(raw string) RTCIceConnectionState {
	switch raw {
	case rtcIceConnectionStateNewStr:
		return RTCIceConnectionStateNew
	case rtcIceConnectionStateCheckingStr:
		return RTCIceConnectionStateChecking
	case rtcIceConnectionStateConnectedStr:
		return RTCIceConnectionStateConnected
	case rtcIceConnectionStateCompletedStr:
		return RTCIceConnectionStateCompleted
	case rtcIceConnectionStateDisconnectedStr:
		return RTCIceConnectionStateDisconnected
	case rtcIceConnectionStateFailedStr:
		return RTCIceConnectionStateFailed
	case rtcIceConnectionStateClosedStr:
		return RTCIceConnectionStateClosed
	default:
		return RTCIceConnectionState(Unknown)
	}
}

func (c RTCIceConnectionState) String() string {
	switch c {
	case RTCIceConnectionStateNew:
		return rtcIceConnectionStateNewStr
	case RTCIceConnectionStateChecking:
		return rtcIceConnectionStateCheckingStr
	case RTCIceConnectionStateConnected:
		return rtcIceConnectionStateConnectedStr
	case RTCIceConnectionStateCompleted:
		return rtcIceConnectionStateCompletedStr
	case RTCIceConnectionStateDisconnected:
		return rtcIceConnectionStateDisconnectedStr
	case RTCIceConnectionStateFailed:
		return rtcIceConnectionStateFailedStr
	case RTCIceConnectionStateClosed:
		return rtcIceConnectionStateClosedStr
	default:
		return ErrUnknownType.Error()
	}
}
