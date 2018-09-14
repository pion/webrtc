package webrtc

// RTCPeerConnectionState indicates the state of the RTCPeerConnection.
type RTCPeerConnectionState int

const (
	// RTCPeerConnectionStateNew indicates that any of the RTCIceTransports or
	// RTCDtlsTransports are in the "new" state and none of the transports are
	// in the "connecting", "checking", "failed" or "disconnected" state, or
	// all transports are in the "closed" state, or there are no transports.
	RTCPeerConnectionStateNew RTCPeerConnectionState = iota + 1

	// RTCPeerConnectionStateConnecting indicates that any of the
	// RTCIceTransports or RTCDtlsTransports are in the "connecting" or
	// "checking" state and none of them is in the "failed" state.
	RTCPeerConnectionStateConnecting

	// RTCPeerConnectionStateConnected indicates that all RTCIceTransports and
	// RTCDtlsTransports are in the "connected", "completed" or "closed" state
	// and at least one of them is in the "connected" or "completed" state.
	RTCPeerConnectionStateConnected

	// RTCPeerConnectionStateDisconnected indicates that any of the
	// RTCIceTransports or RTCDtlsTransports are in the "disconnected" state
	// and none of them are in the "failed" or "connecting" or "checking" state.
	RTCPeerConnectionStateDisconnected

	// RTCPeerConnectionStateFailed indicates that any of the RTCIceTransports
	// or RTCDtlsTransports are in a "failed" state.
	RTCPeerConnectionStateFailed

	// RTCPeerConnectionStateClosed indicates the peer connection is closed
	// and the isClosed member variable of RTCPeerConnection is true.
	RTCPeerConnectionStateClosed
)

// This is done this way because of a linter.
const (
	rtcPeerConnectionStateNewStr          = "new"
	rtcPeerConnectionStateConnectingStr   = "connecting"
	rtcPeerConnectionStateConnectedStr    = "connected"
	rtcPeerConnectionStateDisconnectedStr = "disconnected"
	rtcPeerConnectionStateFailedStr       = "failed"
	rtcPeerConnectionStateClosedStr       = "closed"
)

func newRTCPeerConnectionState(raw string) RTCPeerConnectionState {
	switch raw {
	case rtcPeerConnectionStateNewStr:
		return RTCPeerConnectionStateNew
	case rtcPeerConnectionStateConnectingStr:
		return RTCPeerConnectionStateConnecting
	case rtcPeerConnectionStateConnectedStr:
		return RTCPeerConnectionStateConnected
	case rtcPeerConnectionStateDisconnectedStr:
		return RTCPeerConnectionStateDisconnected
	case rtcPeerConnectionStateFailedStr:
		return RTCPeerConnectionStateFailed
	case rtcPeerConnectionStateClosedStr:
		return RTCPeerConnectionStateClosed
	default:
		return RTCPeerConnectionState(Unknown)
	}
}

func (t RTCPeerConnectionState) String() string {
	switch t {
	case RTCPeerConnectionStateNew:
		return rtcPeerConnectionStateNewStr
	case RTCPeerConnectionStateConnecting:
		return rtcPeerConnectionStateConnectingStr
	case RTCPeerConnectionStateConnected:
		return rtcPeerConnectionStateConnectedStr
	case RTCPeerConnectionStateDisconnected:
		return rtcPeerConnectionStateDisconnectedStr
	case RTCPeerConnectionStateFailed:
		return rtcPeerConnectionStateFailedStr
	case RTCPeerConnectionStateClosed:
		return rtcPeerConnectionStateClosedStr
	default:
		return ErrUnknownType.Error()
	}
}
