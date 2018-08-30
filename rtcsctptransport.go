package webrtc

import "math"

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
	r.MaxChannels = 65535 // TODO: Get from implementation
}
