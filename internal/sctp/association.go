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
	sourcePort                uint16
	destinationPort           uint16
	myMaxNumInboundStreams    uint16
	myMaxNumOutboundStreams   uint16
	myReceiverWindowCredit    uint32
	myCookie                  *paramStateCookie
	payloadQueue              *payloadQueue
	inflightQueue             *payloadQueue
	myMaxMTU                  uint16
	firstSack                 bool
	peerCumulativeTSNAckPoint uint32

	// TODO are these better as channels
	// Put a blocking goroutine in port-recieve (vs callbacks)
	outboundHandler func([]byte)
	dataHandler     func([]byte, uint16)
}

// Push pushes a raw SCTP packet onto the assocation
func (a *Association) Push(raw []byte) error {
	p := &packet{}
	if err := p.unmarshal(raw); err != nil {
		return errors.Wrap(err, "Unable to parse SCTP packet")
	}

	if err := checkPacket(p); err != nil {
		return errors.Wrap(err, "Failed validating packet")
	}

	for _, c := range p.chunks {
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
func NewAssocation(outboundHandler func([]byte), dataHandler func([]byte, uint16)) *Association {
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
		payloadQueue:            &payloadQueue{},
		myMaxMTU:                1200,
		firstSack:               true,
	}
}

func checkPacket(p *packet) error {
	// All packets must adhere to these rules

	// This is the SCTP sender's port number.  It can be used by the
	// receiver in combination with the source IP address, the SCTP
	// destination port, and possibly the destination IP address to
	// identify the association to which this packet belongs.  The port
	// number 0 MUST NOT be used.
	if p.sourcePort == 0 {
		return errors.New("SCTP Packet must not have a source port of 0")
	}

	// This is the SCTP port number to which this packet is destined.
	// The receiving host will use this port number to de-multiplex the
	// SCTP packet to the correct receiving endpoint/application.  The
	// port number 0 MUST NOT be used.
	if p.destinationPort == 0 {
		return errors.New("SCTP Packet must not have a destination port of 0")
	}

	// Check values on the packet that are specific to a particular chunk type
	for _, c := range p.chunks {
		switch c.(type) {
		case *chunkInit:
			// An INIT or INIT ACK chunk MUST NOT be bundled with any other chunk.
			// They MUST be the only chunks present in the SCTP packets that carry
			// them.
			if len(p.chunks) != 1 {
				return errors.New("INIT chunk must not be bundled with any other chunk")
			}

			// A packet containing an INIT chunk MUST have a zero Verification
			// Tag.
			if p.verificationTag != 0 {
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

func (a *Association) handleInit(p *packet, i *chunkInit) (*packet, error) {

	// Should we be setting any of these permanently until we've ACKed further?
	a.myMaxNumInboundStreams = min(i.numInboundStreams, a.myMaxNumInboundStreams)
	a.myMaxNumOutboundStreams = min(i.numOutboundStreams, a.myMaxNumOutboundStreams)
	a.peerVerificationTag = i.initiateTag
	a.sourcePort = p.destinationPort
	a.destinationPort = p.sourcePort

	// 13.2 This is the last TSN received in sequence.  This value
	// is set initially by taking the peer's initial TSN,
	// received in the INIT or INIT ACK chunk, and
	// subtracting one from it.
	a.peerLastTSN = i.initialTSN - 1

	outbound := &packet{}
	outbound.verificationTag = a.peerVerificationTag
	outbound.sourcePort = a.sourcePort
	outbound.destinationPort = a.destinationPort

	initAck := &chunkInitAck{}

	initAck.initialTSN = a.myNextTSN
	initAck.numOutboundStreams = a.myMaxNumOutboundStreams
	initAck.numInboundStreams = a.myMaxNumInboundStreams
	initAck.initiateTag = a.myVerificationTag
	initAck.advertisedReceiverWindowCredit = a.myReceiverWindowCredit

	if a.myCookie == nil {
		a.myCookie = newRandomStateCookie()
	}

	initAck.params = []param{a.myCookie}

	outbound.chunks = []chunk{initAck}

	return outbound, nil

}

func (a *Association) handleData(p *packet, d *chunkPayloadData) (*packet, error) {

	a.payloadQueue.push(d, a.peerLastTSN)

	pd, ok := a.payloadQueue.pop(a.peerLastTSN + 1)
	for ok {
		a.dataHandler(pd.userData, pd.streamIdentifier)
		a.peerLastTSN++
		pd, ok = a.payloadQueue.pop(a.peerLastTSN)
	}

	outbound := &packet{}
	outbound.verificationTag = a.peerVerificationTag
	outbound.sourcePort = a.sourcePort
	outbound.destinationPort = a.destinationPort

	sack := &chunkSelectiveAck{}

	sack.cumulativeTSNAck = a.peerLastTSN
	sack.advertisedReceiverWindowCredit = a.myReceiverWindowCredit
	sack.duplicateTSN = a.payloadQueue.popDuplicates()
	sack.gapAckBlocks = a.payloadQueue.getGapAckBlocks(a.peerLastTSN)
	outbound.chunks = []chunk{sack}

	return outbound, nil

}

func (a *Association) handleSack(p *packet, d *chunkSelectiveAck) ([]*packet, error) {
	// i) If Cumulative TSN Ack is less than the Cumulative TSN Ack
	// Point, then drop the SACK.  Since Cumulative TSN Ack is
	// monotonically increasing, a SACK whose Cumulative TSN Ack is
	// less than the Cumulative TSN Ack Point indicates an out-of-
	// order SACK.
	if a.firstSack {
		a.firstSack = false
		a.peerCumulativeTSNAckPoint = d.cumulativeTSNAck
	}

	// This is an old SACK, toss
	if a.peerCumulativeTSNAckPoint >= d.cumulativeTSNAck {
		return nil, errors.Errorf("SACK Cumulative ACK %v is older than ACK point %v",
			d.cumulativeTSNAck, a.peerCumulativeTSNAckPoint)
	}

	// New ack point, so pop all ACKed packets from inflightQueue
	for i := a.peerCumulativeTSNAckPoint; i <= d.cumulativeTSNAck; i++ {
		_, ok := a.inflightQueue.pop(i)
		if !ok {
			return nil, errors.Errorf("TSN %v unable to be popped from inflight queue", i)
		}
	}
	a.peerCumulativeTSNAckPoint = d.cumulativeTSNAck

	var sackDataPackets []*packet
	var prevEnd uint16
	for _, g := range d.gapAckBlocks {
		for i := prevEnd + 1; i < g.start; i++ {
			pp, ok := a.payloadQueue.get(d.cumulativeTSNAck + uint32(i))
			if !ok {
				return nil, errors.Errorf("Requested non-existent TSN %v", d.cumulativeTSNAck+uint32(i))
			}
			sackDataPackets = append(sackDataPackets, &packet{
				verificationTag: a.peerVerificationTag,
				sourcePort:      a.sourcePort,
				destinationPort: a.destinationPort,
				chunks:          []chunk{pp},
			})
		}
		prevEnd = g.end
	}

	return sackDataPackets, nil
}

func (a *Association) send(p *packet) error {
	raw, err := p.marshal()
	if err != nil {
		return errors.Wrap(err, "Failed to send packet to outbound handler")
	}

	a.outboundHandler(raw)

	return nil
}

// nolint: gocyclo
func (a *Association) handleChunk(p *packet, c chunk) error {
	if _, err := c.check(); err != nil {
		errors.Wrap(err, "Failed validating chunk")
		// TODO: Create ABORT
	}

	switch c := c.(type) {
	case *chunkInit:
		switch a.state {
		case Open:
			pp, err := a.handleInit(p, c)
			if err != nil {
				return errors.Wrap(err, "Failure handling INIT")
			}
			return a.send(pp)
		case CookieEchoed:
			// https://tools.ietf.org/html/rfc4960#section-5.2.1
			// Upon receipt of an INIT in the COOKIE-ECHOED state, an endpoint MUST
			// respond with an INIT ACK using the same parameters it sent in its
			// original INIT chunk (including its Initiate Tag, unchanged)
			return errors.Errorf("TODO respond with original cookie %s", a.state.String())
		default:
			// 5.2.2.  Unexpected INIT in States Other than CLOSED, COOKIE-ECHOED,
			//        COOKIE-WAIT, and SHUTDOWN-ACK-SENT
			return errors.Errorf("TODO Handle Init when in state %s", a.state.String())
		}
	case *chunkAbort:
		fmt.Println("Abort chunk, with errors")
		for _, e := range c.errorCauses {
			fmt.Println(e.errorCauseCode())
		}
	case *chunkHeartbeat:
		hbi, ok := c.params[0].(*paramHeartbeatInfo)
		if !ok {
			fmt.Println("Failed to handle Heartbeat, no ParamHeartbeatInfo")
		}

		return a.send(&packet{
			verificationTag: a.peerVerificationTag,
			sourcePort:      a.sourcePort,
			destinationPort: a.destinationPort,
			chunks: []chunk{&chunkHeartbeatAck{
				params: []param{
					&paramHeartbeatInfo{
						heartbeatInformation: hbi.heartbeatInformation,
					},
				},
			}},
		})
	case *chunkCookieEcho:
		if bytes.Equal(a.myCookie.cookie, c.cookie) {
			return a.send(&packet{
				verificationTag: a.peerVerificationTag,
				sourcePort:      a.sourcePort,
				destinationPort: a.destinationPort,
				chunks:          []chunk{&chunkCookieAck{}},
			})
		}

		// TODO Abort
	case *chunkPayloadData:
		pp, err := a.handleData(p, c)
		if err != nil {
			return errors.Wrap(err, "Failure handling DATA")
		}
		return a.send(pp)
	case *chunkSelectiveAck:
		p, err := a.handleSack(p, c)
		if err != nil {
			return errors.Wrap(err, "Failure handling SACK")
		}
		for _, pp := range p {
			err := a.send(pp)
			if err != nil {
				return errors.Wrap(err, "Failure handling SACK")
			}
		}
	}

	return nil
}
