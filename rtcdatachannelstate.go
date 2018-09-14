package webrtc

// RTCDataChannelState indicates the state of a data channel.
type RTCDataChannelState int

const (
	// RTCDataChannelStateConnecting indicates that the data channel is being
	// established. This is the initial state of RTCDataChannel, whether created
	// with CreateDataChannel, or dispatched as a part of an RTCDataChannelEvent.
	RTCDataChannelStateConnecting RTCDataChannelState = iota + 1

	// RTCDataChannelStateOpen indicates that the underlying data transport is
	// established and communication is possible.
	RTCDataChannelStateOpen

	// RTCDataChannelStateClosing indicates that the procedure to close down the
	// underlying data transport has started.
	RTCDataChannelStateClosing

	// RTCDataChannelStateClosed indicates that the underlying data transport
	// has been closed or could not be established.
	RTCDataChannelStateClosed
)

// This is done this way because of a linter.
const (
	rtcDataChannelStateConnectingStr = "connecting"
	rtcDataChannelStateOpenStr       = "open"
	rtcDataChannelStateClosingStr    = "closing"
	rtcDataChannelStateClosedStr     = "closed"
)

func newRTCDataChannelState(raw string) RTCDataChannelState {
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
