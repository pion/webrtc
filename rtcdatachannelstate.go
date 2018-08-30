package webrtc

// RTCPeerConnectionState indicates the state of the RTCPeerConnection.
type RTCDataChannelState int

const (
	RTCDataChannelStateConnecting RTCDataChannelState = iota + 1
	RTCDataChannelStateOpen
	RTCDataChannelStateClosing
	RTCDataChannelStateClosed
)

// This is done this way because of a linter.
const (
	rtcDataChannelStateConnectingStr = "connecting"
	rtcDataChannelStateOpenStr       = "open"
	rtcDataChannelStateClosingStr    = "closing"
	rtcDataChannelStateClosedStr     = "closed"
)

// NewRTCDataChannelState defines a procedure for creating a new
// RTCDataChannelState from a raw string naming the data chanenel state.
func NewRTCDataChannelState(raw string) RTCDataChannelState {
	switch raw {
	case rtcDataChannelStateConnectingStr:
		return RTCDataChannelStateConnecting
	case rtcDataChannelStateOpenStr:
		return RTCDataChannelStateOpen
	case rtcDataChannelStateClosingStr:
		return RTCDataChannelStateClosing
	case rtcDataChannelStateClosedStr:
		return RTCDataChannelStateClosed
	default:
		return RTCDataChannelState(Unknown)
	}
}

func (t RTCDataChannelState) String() string {
	switch t {
	case RTCDataChannelStateConnecting:
		return rtcDataChannelStateConnectingStr
	case RTCDataChannelStateOpen:
		return rtcDataChannelStateOpenStr
	case RTCDataChannelStateClosing:
		return rtcDataChannelStateClosingStr
	case RTCDataChannelStateClosed:
		return rtcDataChannelStateClosedStr
	default:
		return ErrUnknownType.Error()
	}
}
