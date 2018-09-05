package webrtc

import (
	"math"
)

// RTCSctpTransport provides details about the SCTP transport.
type RTCSctpTransport struct {
	// Transport represents the transport over which all SCTP packets for data
	// channels will be sent and received.
	Transport *RTCDtlsTransport

	// State represents the current state of the SCTP transport.
	State RTCSctpTransportState

	// MaxMessageSize represents the maximum size of data that can be passed to
	// RTCDataChannel's send() method.
	MaxMessageSize float64

	// MaxChannels represents the maximum amount of RTCDataChannel's that can
	// be used simultaneously.
	MaxChannels *uint16

	// OnStateChange  func()

	// dataChannels
	// dataChannels map[uint16]*RTCDataChannel
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
	val := uint16(65535)
	r.MaxChannels = &val // TODO: Get from implementation
}
