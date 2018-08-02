package webrtc

import "math"

// RTCSctpTransportState indicates the state of the SCTP transport.
type RTCSctpTransportState int

const (
	// RTCSctpTransportStateConnecting indicates the RTCSctpTransport is in the process of negotiating an association.
	RTCSctpTransportStateConnecting RTCSctpTransportState = iota + 1

	// RTCSctpTransportStateConnected indicates the negotiation of an association is completed.
	RTCSctpTransportStateConnected

	// RTCSctpTransportStateClosed indicates a SHUTDOWN or ABORT chunk is received or when the SCTP association has been closed intentionally.
	RTCSctpTransportStateClosed
)

func (s RTCSctpTransportState) String() string {
	switch s {
	case RTCSctpTransportStateConnecting:
		return "connecting"
	case RTCSctpTransportStateConnected:
		return "connected"
	case RTCSctpTransportStateClosed:
		return "closed"
	default:
		return "Unknown"
	}
}

// RTCSctpTransport provides details about the SCTP transport.
type RTCSctpTransport struct {
	State RTCSctpTransportState // TODO: Set RTCSctpTransportState
	// transport *RTCDtlsTransport // TODO: DTLS introspection API
	MaxMessageSize float64
	MaxChannels    uint16
	// onstatechange func()
}

func newRTCSctpTransport() *RTCSctpTransport {
	res := &RTCSctpTransport{
		State: RTCSctpTransportStateConnecting,
	}

	res.updateMessageSize()
	res.updateMaxChannels()

	return res
}

func (r *RTCSctpTransport) updateMessageSize() {
	var remoteMaxMessageSize float64 = 65536 // TODO: get from SDP
	var canSendSize float64 = 65536          // TODO: Get from SCTP implementation

	r.MaxMessageSize = r.calcMessageSize(remoteMaxMessageSize, canSendSize)
}

func (r *RTCSctpTransport) calcMessageSize(remoteMaxMessageSize, canSendSize float64) float64 {
	switch {
	case remoteMaxMessageSize == 0 &&
		canSendSize == 0:
		return math.Inf(1)

	case remoteMaxMessageSize == 0:
		return canSendSize

	case canSendSize == 0:
		return remoteMaxMessageSize

	case canSendSize > remoteMaxMessageSize:
		return remoteMaxMessageSize

	default:
		return canSendSize
	}
}

func (r *RTCSctpTransport) updateMaxChannels() {
	r.maxChannels = 65536 // TODO: Get from implementation
}
