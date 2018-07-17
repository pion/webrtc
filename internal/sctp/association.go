package sctp

import (
	"bytes"
	"fmt"

	"math"
	"math/rand"
	"time"

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
	case Open:
		return "Open"
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
	peerVerificationTag uint32
	myVerificationTag   uint32
	state               AssociationState
	//peerTransportList
	//primaryPath
	//overallErrorCount
	//overallErrorThreshold
	//peerReceiverWindow (peerRwnd)
	myNextTSN   uint32 // nextTSN
	peerLastTSN uint32 // lastRcvdTSN
	//peerMissingTSN (MappingArray)
	//ackState
	//inboundStreams
	//outboundStreams
	//reassemblyQueue
	//localTransportAddressList
	//associationPTMU

	// Non-RFC internal data
	sourcePort              uint16
	destinationPort         uint16
	myMaxNumInboundStreams  uint16
	myMaxNumOutboundStreams uint16
	myReceiverWindowCredit  uint32
	myCookie                *ParamStateCookie
	payloadQueue            *PayloadQueue
	myMaxMTU                uint16

	// TODO are these better as channels
	// Put a blocking goroutine in port-recieve (vs callbacks)
	outboundHandler func(*Packet)
	dataHandler     func([]byte, uint16)
}

// PushPacket pushes a SCTP packet onto the assocation
func (a *Association) PushPacket(p *Packet) error {
	if err := checkPacket(p); err != nil {
		return errors.Wrap(err, "Failed validating packet")
	}

	for _, c := range p.Chunks {
		if err := a.handleChunk(p, c); err != nil {
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
func NewAssocation(outboundHandler func(*Packet), dataHandler func([]byte, uint16)) *Association {
	rs := rand.NewSource(time.Now().UnixNano())
	r := rand.New(rs)

	return &Association{
		myVerificationTag: r.Uint32(),
		myNextTSN:         r.Uint32(),
		outboundHandler:   outboundHandler,
		dataHandler:       dataHandler,
		state:             Open,
		myMaxNumOutboundStreams: math.MaxUint16,
		myMaxNumInboundStreams:  math.MaxUint16,
		myReceiverWindowCredit:  10 * 1500, // 10 Max MTU packets buffer
		payloadQueue:            &PayloadQueue{},
		myMaxMTU:                1200,
	}
}

func checkPacket(p *Packet) error {
	// All packets must adhere to these rules

	// This is the SCTP sender's port number.  It can be used by the
	// receiver in combination with the source IP address, the SCTP
	// destination port, and possibly the destination IP address to
	// identify the association to which this packet belongs.  The port
	// number 0 MUST NOT be used.
	if p.SourcePort == 0 {
		return errors.New("SCTP Packet must not have a source port of 0")
	}

	// This is the SCTP port number to which this packet is destined.
	// The receiving host will use this port number to de-multiplex the
	// SCTP packet to the correct receiving endpoint/application.  The
	// port number 0 MUST NOT be used.
	if p.DestinationPort == 0 {
		return errors.New("SCTP Packet must not have a destination port of 0")
	}

	// Check values on the packet that are specific to a particular chunk type
	for _, c := range p.Chunks {
		switch c.(type) {
		case *Init:
			// An INIT or INIT ACK chunk MUST NOT be bundled with any other chunk.
			// They MUST be the only chunks present in the SCTP packets that carry
			// them.
			if len(p.Chunks) != 1 {
				return errors.New("INIT chunk must not be bundled with any other chunk")
			}

			// A packet containing an INIT chunk MUST have a zero Verification
			// Tag.
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

func (a *Association) handleInit(p *Packet, i *Init) (*Packet, error) {

	// Should we be setting any of these permanently until we've ACKed further?
	a.myMaxNumInboundStreams = min(i.numInboundStreams, a.myMaxNumInboundStreams)
	a.myMaxNumOutboundStreams = min(i.numOutboundStreams, a.myMaxNumOutboundStreams)
	a.peerVerificationTag = i.initiateTag
	a.sourcePort = p.DestinationPort
	a.destinationPort = p.SourcePort

	// 13.2 This is the last TSN received in sequence.  This value
	// is set initially by taking the peer's initial TSN,
	// received in the INIT or INIT ACK chunk, and
	// subtracting one from it.
	a.peerLastTSN = i.initialTSN - 1

	outbound := &Packet{}
	outbound.VerificationTag = a.peerVerificationTag
	outbound.SourcePort = a.sourcePort
	outbound.DestinationPort = a.destinationPort

	initAck := &InitAck{}

	initAck.initialTSN = a.myNextTSN
	initAck.numOutboundStreams = a.myMaxNumOutboundStreams
	initAck.numInboundStreams = a.myMaxNumInboundStreams
	initAck.initiateTag = a.myVerificationTag
	initAck.advertisedReceiverWindowCredit = a.myReceiverWindowCredit

	if a.myCookie == nil {
		a.myCookie = NewRandomStateCookie()
	}

	initAck.params = []Param{a.myCookie}

	outbound.Chunks = []Chunk{initAck}

	return outbound, nil

}

func (a *Association) handleData(p *Packet, d *PayloadData) (*Packet, error) {

	a.payloadQueue.Push(d, a.peerLastTSN)

	pd, ok := a.payloadQueue.Pop(a.peerLastTSN + 1)
	for ok {
		a.dataHandler(pd.userData, pd.streamIdentifier)
		a.peerLastTSN++
		pd, ok = a.payloadQueue.Pop(a.peerLastTSN + 1)
	}

	outbound := &Packet{}
	outbound.VerificationTag = a.peerVerificationTag
	outbound.SourcePort = a.sourcePort
	outbound.DestinationPort = a.destinationPort

	sack := &SelectiveAck{}

	sack.cumulativeTSNAck = a.peerLastTSN
	sack.advertisedReceiverWindowCredit = a.myReceiverWindowCredit
	sack.duplicateTSN = a.payloadQueue.PopDuplicates()
	sack.gapAckBlocks = a.payloadQueue.GetGapAckBlocks(a.peerLastTSN)
	outbound.Chunks = []Chunk{sack}

	return outbound, nil

}

func (a *Association) handleChunk(p *Packet, c Chunk) error {
	if _, err := c.Check(); err != nil {
		errors.Wrap(err, "Failed validating chunk")
		// TODO: Create ABORT
	}

	switch c := c.(type) {
	case *Init:
		switch a.state {
		case Open:
			p, err := a.handleInit(p, c)
			if err != nil {
				return errors.Wrap(err, "Failure handling INIT")
			}
			a.outboundHandler(p)
			return nil
		case CookieEchoed:
			// https://tools.ietf.org/html/rfc4960#section-5.2.1
			// Upon receipt of an INIT in the COOKIE-ECHOED state, an endpoint MUST
			// respond with an INIT ACK using the same parameters it sent in its
			// original INIT chunk (including its Initiate Tag, unchanged)
			return errors.Errorf("TODO respond with original cookie %s", a.state)
		default:
			// 5.2.2.  Unexpected INIT in States Other than CLOSED, COOKIE-ECHOED,
			//        COOKIE-WAIT, and SHUTDOWN-ACK-SENT
			return errors.Errorf("TODO Handle Init when in state %s", a.state)
		}
	case *Abort:
		fmt.Println("Abort chunk, with errors")
		for _, e := range c.ErrorCauses {
			fmt.Println(e.errorCauseCode())
		}
	case *Heartbeat:
		hbi, ok := c.params[0].(*ParamHeartbeatInfo)
		if !ok {
			fmt.Println("Failed to handle Heartbeat, no ParamHeartbeatInfo")
		}

		a.outboundHandler(&Packet{
			VerificationTag: a.peerVerificationTag,
			SourcePort:      a.sourcePort,
			DestinationPort: a.destinationPort,
			Chunks: []Chunk{&HeartbeatAck{
				params: []Param{
					&ParamHeartbeatInfo{
						HeartbeatInformation: hbi.HeartbeatInformation,
					},
				},
			}},
		})
	case *CookieEcho:
		if bytes.Equal(a.myCookie.Cookie, c.Cookie) {
			a.outboundHandler(&Packet{
				VerificationTag: a.peerVerificationTag,
				SourcePort:      a.sourcePort,
				DestinationPort: a.destinationPort,
				Chunks:          []Chunk{&CookieAck{}},
			})
		} else {
			// TODO Abort
		}
	case *PayloadData:
		p, err := a.handleData(p, c)
		if err != nil {
			return errors.Wrap(err, "Failure handling DATA")
		}
		a.outboundHandler(p)
	}

	return nil
}
