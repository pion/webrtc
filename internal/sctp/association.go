package sctp

import "github.com/pkg/errors"

// https://tools.ietf.org/html/rfc4960#section-13.2
type AssociationState uint8

const (
	CookieWait AssociationState = iota
	CookieEchoed
	Established
	ShutdownPending
	ShutdownSent
	ShutdownReceived
	ShutdownAckSent
)

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
}

func hasPacket(p *Packet, t ChunkType) bool {
	for _, c := range p.Chunks {
		if c.Type() == t {
			return true
		}
	}

	return false
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

func (a *Association) PushPacket(p *Packet) error {
	err := checkPacket(p)
	if err != nil {
		return errors.Wrap(err, "Failed validating packet")
	}

	for _, c := range p.Chunks {
		a.handleChunk(c)
	}

	return nil
}

func (a *Association) handleChunk(c Chunk) error {
	err := c.Check()
	if err != nil {
		errors.Wrap(err, "Failed validating chunk")
		// TODO: Create ABORT
	}

	switch ct := c.(type) {
	case *Init:
		if a.State != CookieEchoed && a.State != CookieWait {
			// https://tools.ietf.org/html/rfc4960#section-5.2.1
			// 5.2.1.  INIT Received in COOKIE-WAIT or COOKIE-ECHOED State (Item B)
			a.myMaxNumInboundStreams = min(ct.numInboundStreams, a.myMaxNumInboundStreams)
			a.myMaxNumOutboundStreams = min(ct.numOutboundStreams, a.myMaxNumOutboundStreams)
		} else {
			//https://tools.ietf.org/html/rfc4960#section-5.2.2
			// 5.2.2.  Unexpected INIT in States Other than CLOSED, COOKIE-ECHOED,
			//        COOKIE-WAIT, and SHUTDOWN-ACK-SENT
		}

	}

	return nil
}
