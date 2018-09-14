package webrtc

// RTCDtlsTransportState indicates the dtsl transport establishment state.
type RTCDtlsTransportState int

const (
	// RTCDtlsTransportStateNew indicates that DTLS has not started negotiating
	// yet.
	RTCDtlsTransportStateNew RTCDtlsTransportState = iota + 1

	// RTCDtlsTransportStateConnecting indicates that DTLS is in the process of
	// negotiating a secure connection and verifying the remote fingerprint.
	RTCDtlsTransportStateConnecting

	// RTCDtlsTransportStateConnected indicates that DTLS has completed
	// negotiation of a secure connection and verified the remote fingerprint.
	RTCDtlsTransportStateConnected

	// RTCDtlsTransportStateClosed indicates that the transport has been closed
	// intentionally as the result of receipt of a close_notify alert, or
	// calling close().
	RTCDtlsTransportStateClosed

	// RTCDtlsTransportStateFailed indicates that the transport has failed as
	// the result of an error (such as receipt of an error alert or failure to
	// validate the remote fingerprint).
	RTCDtlsTransportStateFailed
)

// This is done this way because of a linter.
const (
	rtcDtlsTransportStateNewStr        = "new"
	rtcDtlsTransportStateConnectingStr = "connecting"
	rtcDtlsTransportStateConnectedStr  = "connected"
	rtcDtlsTransportStateClosedStr     = "closed"
	rtcDtlsTransportStateFailedStr     = "failed"
)

func newRTCDtlsTransportState(raw string) RTCDtlsTransportState {
	switch raw {
	case rtcDtlsTransportStateNewStr:
		return RTCDtlsTransportStateNew
	case rtcDtlsTransportStateConnectingStr:
		return RTCDtlsTransportStateConnecting
	case rtcDtlsTransportStateConnectedStr:
		return RTCDtlsTransportStateConnected
	case rtcDtlsTransportStateClosedStr:
		return RTCDtlsTransportStateClosed
	case rtcDtlsTransportStateFailedStr:
		return RTCDtlsTransportStateFailed
	default:
		return RTCDtlsTransportState(Unknown)
	}
}

func (t RTCDtlsTransportState) String() string {
	switch t {
	case RTCDtlsTransportStateNew:
		return rtcDtlsTransportStateNewStr
	case RTCDtlsTransportStateConnecting:
		return rtcDtlsTransportStateConnectingStr
	case RTCDtlsTransportStateConnected:
		return rtcDtlsTransportStateConnectedStr
	case RTCDtlsTransportStateClosed:
		return rtcDtlsTransportStateClosedStr
	case RTCDtlsTransportStateFailed:
		return rtcDtlsTransportStateFailedStr
	default:
		return ErrUnknownType.Error()
	}
}
