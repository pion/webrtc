package ice

import "github.com/pion/ice"

// TransportState represents the current state of the ICE transport.
type TransportState int

const (
	// TransportStateNew indicates the Transport is waiting
	// for remote candidates to be supplied.
	TransportStateNew = iota + 1

	// TransportStateChecking indicates the Transport has
	// received at least one remote candidate, and a local and remote
	// ICECandidateComplete dictionary was not added as the last candidate.
	TransportStateChecking

	// TransportStateConnected indicates the Transport has
	// received a response to an outgoing connectivity check, or has
	// received incoming DTLS/media after a successful response to an
	// incoming connectivity check, but is still checking other candidate
	// pairs to see if there is a better connection.
	TransportStateConnected

	// TransportStateCompleted indicates the Transport tested
	// all appropriate candidate pairs and at least one functioning
	// candidate pair has been found.
	TransportStateCompleted

	// TransportStateFailed indicates the Transport the last
	// candidate was added and all appropriate candidate pairs have either
	// failed connectivity checks or have lost consent.
	TransportStateFailed

	// TransportStateDisconnected indicates the Transport has received
	// at least one local and remote candidate, but the final candidate was
	// received yet and all appropriate candidate pairs thus far have been
	// tested and failed.
	TransportStateDisconnected

	// TransportStateClosed indicates the Transport has shut down
	// and is no longer responding to STUN requests.
	TransportStateClosed
)

func (c TransportState) String() string {
	switch c {
	case TransportStateNew:
		return "new"
	case TransportStateChecking:
		return "checking"
	case TransportStateConnected:
		return "connected"
	case TransportStateCompleted:
		return "completed"
	case TransportStateFailed:
		return "failed"
	case TransportStateDisconnected:
		return "disconnected"
	case TransportStateClosed:
		return "closed"
	default:
		return unknownStr
	}
}

func newTransportStateFromICE(i ice.ConnectionState) TransportState {
	switch i {
	case ice.ConnectionStateNew:
		return TransportStateNew
	case ice.ConnectionStateChecking:
		return TransportStateChecking
	case ice.ConnectionStateConnected:
		return TransportStateConnected
	case ice.ConnectionStateCompleted:
		return TransportStateCompleted
	case ice.ConnectionStateFailed:
		return TransportStateFailed
	case ice.ConnectionStateDisconnected:
		return TransportStateDisconnected
	case ice.ConnectionStateClosed:
		return TransportStateClosed
	default:
		return TransportState(Unknown)
	}
}

func (c TransportState) toICE() ice.ConnectionState {
	switch c {
	case TransportStateNew:
		return ice.ConnectionStateNew
	case TransportStateChecking:
		return ice.ConnectionStateChecking
	case TransportStateConnected:
		return ice.ConnectionStateConnected
	case TransportStateCompleted:
		return ice.ConnectionStateCompleted
	case TransportStateFailed:
		return ice.ConnectionStateFailed
	case TransportStateDisconnected:
		return ice.ConnectionStateDisconnected
	case TransportStateClosed:
		return ice.ConnectionStateClosed
	default:
		return ice.ConnectionState(Unknown)
	}

}
