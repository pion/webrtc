package webrtc

import "github.com/pions/webrtc/pkg/ice"

// RTCIceTransportState represents the current state of the ICE transport.
type RTCIceTransportState int

const (
	// RTCIceTransportStateNew indicates the RTCIceTransport is waiting
	// for remote candidates to be supplied.
	RTCIceTransportStateNew = iota + 1

	// RTCIceTransportStateChecking indicates the RTCIceTransport has
	// received at least one remote candidate, and a local and remote
	// RTCIceCandidateComplete dictionary was not added as the last candidate.
	RTCIceTransportStateChecking

	// RTCIceTransportStateConnected indicates the RTCIceTransport has
	// received a response to an outgoing connectivity check, or has
	// received incoming DTLS/media after a successful response to an
	// incoming connectivity check, but is still checking other candidate
	// pairs to see if there is a better connection.
	RTCIceTransportStateConnected

	// RTCIceTransportStateCompleted indicates the RTCIceTransport tested
	// all appropriate candidate pairs and at least one functioning
	// candidate pair has been found.
	RTCIceTransportStateCompleted

	// RTCIceTransportStateFailed indicates the RTCIceTransport the last
	// candidate was added and all appropriate candidate pairs have either
	// failed connectivity checks or have lost consent.
	RTCIceTransportStateFailed

	// RTCIceTransportStateDisconnected indicates the RTCIceTransport has received
	// at least one local and remote candidate, but the final candidate was
	// received yet and all appropriate candidate pairs thus far have been
	// tested and failed.
	RTCIceTransportStateDisconnected

	// RTCIceTransportStateClosed indicates the RTCIceTransport has shut down
	// and is no longer responding to STUN requests.
	RTCIceTransportStateClosed
)

func (c RTCIceTransportState) String() string {
	switch c {
	case RTCIceTransportStateNew:
		return "new"
	case RTCIceTransportStateChecking:
		return "checking"
	case RTCIceTransportStateConnected:
		return "connected"
	case RTCIceTransportStateCompleted:
		return "completed"
	case RTCIceTransportStateFailed:
		return "failed"
	case RTCIceTransportStateDisconnected:
		return "disconnected"
	case RTCIceTransportStateClosed:
		return "closed"
	default:
		return "invalid"
	}
}

func newRTCIceTransportStateFromICE(i ice.ConnectionState) RTCIceTransportState {
	switch i {
	case ice.ConnectionStateNew:
		return RTCIceTransportStateNew
	case ice.ConnectionStateChecking:
		return RTCIceTransportStateChecking
	case ice.ConnectionStateConnected:
		return RTCIceTransportStateConnected
	case ice.ConnectionStateCompleted:
		return RTCIceTransportStateCompleted
	case ice.ConnectionStateFailed:
		return RTCIceTransportStateFailed
	case ice.ConnectionStateDisconnected:
		return RTCIceTransportStateDisconnected
	case ice.ConnectionStateClosed:
		return RTCIceTransportStateClosed
	default:
		return RTCIceTransportState(0)
	}
}

func (c RTCIceTransportState) toICE() ice.ConnectionState {
	switch c {
	case RTCIceTransportStateNew:
		return ice.ConnectionStateNew
	case RTCIceTransportStateChecking:
		return ice.ConnectionStateChecking
	case RTCIceTransportStateConnected:
		return ice.ConnectionStateConnected
	case RTCIceTransportStateCompleted:
		return ice.ConnectionStateCompleted
	case RTCIceTransportStateFailed:
		return ice.ConnectionStateFailed
	case RTCIceTransportStateDisconnected:
		return ice.ConnectionStateDisconnected
	case RTCIceTransportStateClosed:
		return ice.ConnectionStateClosed
	default:
		return ice.ConnectionState(0)
	}

}
