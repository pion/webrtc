package sctp

import (
	"fmt"

	"github.com/pkg/errors"
)

// AssociationState is an enum for the states that an Association will transition
// through while connecting
// https://tools.ietf.org/html/rfc4960#section-13.2
type AssociationState uint8

// AssociationState enums
const (
	Open AssociationState = iota + 1
	CookieEchoed
	CookieWait
	Established
	ShutdownAckSent
	ShutdownPending
	ShutdownReceived
	ShutdownSent
)

func (a AssociationState) String() string {
	switch a {
	case CookieEchoed:
		return "CookieEchoed"
	case CookieWait:
		return "CookieWait"
	case Established:
		return "Established"
	case ShutdownPending:
		return "ShutdownPending"
	case ShutdownSent:
		return "ShutdownSent"
	case ShutdownReceived:
		return "ShutdownReceived"
	case ShutdownAckSent:
		return "ShutdownAckSent"
	default:
		return fmt.Sprintf("Invalid AssociationState %d", a)
	}
}

// Association represents an SCTP assocation
// 13.2.  Parameters Necessary per Association (i.e., the TCB)
// Peer        : Tag value to be sent in every packet and is received
// Verification: in the INIT or INIT ACK chunk.
// Tag         :
//
// My          : Tag expected in every inbound packet and sent in the
// Verification: INIT or INIT ACK chunk.
//
// Tag         :
// State       : A state variable indicating what state the association
//             : is in, i.e., COOKIE-WAIT, COOKIE-ECHOED, ESTABLISHED,
//             : SHUTDOWN-PENDING, SHUTDOWN-SENT, SHUTDOWN-RECEIVED,
//             : SHUTDOWN-ACK-SENT.
//
//               Note: No "CLOSED" state is illustrated since if a
//               association is "CLOSED" its TCB SHOULD be removed.
type Association struct {
	PeerVerificationTag uint32
	MyVerificationTag   uint32
	State               AssociationState

	// Non-RFC internal data
	myMaxNumInboundStreams  uint16
	myMaxNumOutboundStreams uint16

	outboundHandler func(*Packet)
	dataHandler     func([]byte)
}

// PushPacket pushes a SCTP packet onto the assocation
func (a *Association) PushPacket(p *Packet) error {
	if err := checkPacket(p); err != nil {
		return errors.Wrap(err, "Failed validating packet")
	}

	for _, c := range p.Chunks {
		if err := a.handleChunk(c); err != nil {
			return errors.Wrap(err, "Failed handling chunk")
		}
	}

	return nil
}

// Close ends the SCTP Association and cleans up any state
func (a *Association) Close() error {
	return nil
}

// NewAssocation creates a new Association and the state needed to manage it
func NewAssocation(outboundHandler func(*Packet), dataHandler func([]byte)) *Association {
	return &Association{
		outboundHandler: outboundHandler,
		dataHandler:     dataHandler,
		State:           Open,
	}
}

func checkPacket(p *Packet) error {
	for _, c := range p.Chunks {
		switch c.(type) {
		case *Init:
			// An INIT or INIT ACK chunk MUST NOT be bundled with any other chunk.
			// They MUST be the only chunks present in the SCTP packets that carry
			// them.
			if len(p.Chunks) != 1 {
				return errors.New("INIT chunk must not be bundled with any other chunk")
			}

			if p.VerificationTag != 0 {
				return errors.Errorf("INIT chunk expects a verification tag of 0 on the packet when out-of-the-blue")
			}
		}
	}

	return nil
}

func min(a, b uint16) uint16 {
	if a < b {
		return a
	}
	return b
}

func (a *Association) handleChunk(c Chunk) error {
	if err := c.Check(); err != nil {
		errors.Wrap(err, "Failed validating chunk")
		// TODO: Create ABORT
	}

	switch ct := c.(type) {
	case *Init:
		switch a.State {
		case Open:
			a.myMaxNumInboundStreams = min(ct.numInboundStreams, a.myMaxNumInboundStreams)
			a.myMaxNumOutboundStreams = min(ct.numOutboundStreams, a.myMaxNumOutboundStreams)
			a.State = CookieEchoed
		case CookieEchoed:
			// https://tools.ietf.org/html/rfc4960#section-5.2.1
			// Upon receipt of an INIT in the COOKIE-ECHOED state, an endpoint MUST
			// respond with an INIT ACK using the same parameters it sent in its
			// original INIT chunk (including its Initiate Tag, unchanged)
			return errors.Errorf("TODO respond with original cookie %s", a.State)
		default:
			// 5.2.2.  Unexpected INIT in States Other than CLOSED, COOKIE-ECHOED,
			//        COOKIE-WAIT, and SHUTDOWN-ACK-SENT
			return errors.Errorf("TODO Handle Init when in state %s", a.State)
		}
	}

	return nil
}
