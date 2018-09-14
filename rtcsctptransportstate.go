package webrtc

// RTCSctpTransportState indicates the state of the SCTP transport.
type RTCSctpTransportState int

const (
	// RTCSctpTransportStateConnecting indicates the RTCSctpTransport is in the
	// process of negotiating an association. This is the initial state of the
	// SctpTransportState when an RTCSctpTransport is created.
	RTCSctpTransportStateConnecting RTCSctpTransportState = iota + 1

	// RTCSctpTransportStateConnected indicates the negotiation of an
	// association is completed.
	RTCSctpTransportStateConnected

	// RTCSctpTransportStateClosed indicates a SHUTDOWN or ABORT chunk is
	// received or when the SCTP association has been closed intentionally,
	// such as by closing the peer connection or applying a remote description
	// that rejects data or changes the SCTP port.
	RTCSctpTransportStateClosed
)

// This is done this way because of a linter.
const (
	rtcSctpTransportStateConnectingStr = "connecting"
	rtcSctpTransportStateConnectedStr  = "connected"
	rtcSctpTransportStateClosedStr     = "closed"
)

func newRTCSctpTransportState(raw string) RTCSctpTransportState {
	switch raw {
	case rtcSctpTransportStateConnectingStr:
		return RTCSctpTransportStateConnecting
	case rtcSctpTransportStateConnectedStr:
		return RTCSctpTransportStateConnected
	case rtcSctpTransportStateClosedStr:
		return RTCSctpTransportStateClosed
	default:
		return RTCSctpTransportState(Unknown)
	}
}

func (s RTCSctpTransportState) String() string {
	switch s {
	case RTCSctpTransportStateConnecting:
		return rtcSctpTransportStateConnectingStr
	case RTCSctpTransportStateConnected:
		return rtcSctpTransportStateConnectedStr
	case RTCSctpTransportStateClosed:
		return rtcSctpTransportStateClosedStr
	default:
		return ErrUnknownType.Error()
	}
}
