package sctp

import (
	"bytes"
	"fmt"
	"sync"

	"container/list"
	"github.com/pkg/errors"
	"math"
	"math/rand"
	"time"
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

// Association represents an SCTP association
// 13.2.  Parameters Necessary per Association (i.e., the TCB)
// Peer        : Tag value to be sent in every packet and is received
// Verification: in the INIT or INIT ACK chunk.
// Tag         :
//
// My          : Tag expected in every reader packet and sent in the
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
	sync.Mutex

	peerVerificationTag uint32
	myVerificationTag   uint32
	state               AssociationState
	// peerTransportList
	// primaryPath
	// overallErrorCount
	// overallErrorThreshold
	// peerReceiverWindow (peerRwnd)
	myNextTSN   uint32 // nextTSN
	peerLastTSN uint32 // lastRcvdTSN
	// peerMissingTSN (MappingArray)
	// ackState
	// inboundStreams
	// outboundStreams
	// reassemblyQueue
	// localTransportAddressList
	// associationPTMU

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
	reassemblyQueue           map[uint16]*reassemblyQueue
	outboundStreams           map[uint16]uint16

	// TODO are these better as channels
	// Put a blocking goroutine in port-receive (vs callbacks)
	// outboundHandler func([]byte)
	// dataHandler     func([]byte, uint16, PayloadProtocolID)

	OnReceive func(event ReceiveEvent)
	// OnSendFailure func()
	// OnNetworkStatusChange func()
	OnCommunicationUp func(event CommunicationUpEvent)
	// OnCommunicationLost func()
	// OnCommunicationError func()
	// OnRestart func()
	// OnShutdownComplete func()

	Input  chan interface{}
	reader chan interface{}

	Output chan interface{}
	writer chan interface{}
}

// NewAssocation creates a new Association and the state needed to manage it
func NewAssocation() *Association {
	rs := rand.NewSource(time.Now().UnixNano())
	r := rand.New(rs)

	association := &Association{
		myMaxNumOutboundStreams: math.MaxUint16,
		myMaxNumInboundStreams:  math.MaxUint16,
		myReceiverWindowCredit:  10 * 1500, // 10 Max MTU packets writer
		payloadQueue:            &payloadQueue{},
		inflightQueue:           &payloadQueue{},
		myMaxMTU:                1200,
		firstSack:               true,
		reassemblyQueue:         make(map[uint16]*reassemblyQueue),
		outboundStreams:         make(map[uint16]uint16),
		myVerificationTag:       r.Uint32(),
		myNextTSN:               r.Uint32(),
		state:                   Open,
		Input:                   make(chan interface{}, 1),
		Output:                  make(chan interface{}, 1),
		reader:                  make(chan interface{}, 1),
		writer:                  make(chan interface{}, 1),
	}

	go association.inboundHander()
	go association.outboundHandler()
	return association
}

func (a *Association) outboundHandler() {
	queue := list.New()
	for {
		if front := queue.Front(); front == nil {
			if a.writer == nil {
				close(a.Output)
				return
			}
			value, ok := <-a.writer
			if !ok {
				close(a.Output)
				return
			}
			queue.PushBack(value)
		} else {
			select {
			case a.Output <- front.Value:
				queue.Remove(front)
			case value, ok := <-a.writer:
				if ok {
					queue.PushBack(value)
				} else {
					a.writer = nil
				}
			}
		}
	}
}

func (a *Association) inboundHander() {
	queue := list.New()
	for {
		if front := queue.Front(); front == nil {
			if a.Input == nil {
				close(a.reader)
				return
			}

			value, ok := <-a.Input
			if !ok {
				close(a.reader)
				return
			}
			queue.PushBack(value)
		} else {
			select {
			case a.reader <- front.Value:
				// val := a.reader
				// switch val := val.(type) {
				// case sctp.ReceiveEvent:
				// 	go c.onReceiveHandler(val)
				// 	// case sctp.SendFailureEvent:
				// 	// case sctp.NetworkStatusChangeEvent:
				// case sctp.CommunicationUpEvent:
				// 	go c.onCommunicationUpHandler(val)
				// 	// case sctp.CommunicationLostEvent:
				// 	// case sctp.CommunicationErrorEvent
				// 	// case sctp.RestartEvent
				// 	// case sctp.ShutdownCompleteEvent:
				// }
				queue.Remove(front)
			case value, ok := <-a.Input:
				if ok {
					queue.PushBack(value)
				} else {
					a.Input = nil
				}
			}
		}
	}
}

// FromDtls is a syntactic sugar function for the Input channel. It is useful
// for readability when working against a dtls as lower protocol layer. The
// function name is more descriptive of the source of data.
func (a *Association) FromDtls() chan interface{} {
	return a.Input
}

// ToDtls is a syntactic sugar function for the Output channel. If is useful
// for readability when working against a dtls as lower protocol layer. The
// function name is more descriptive of the destination for data.
func (a *Association) ToDtls() chan interface{} {
	return a.Output
}

func (a *Association) Associate(outboundStreamCount uint16) {
	if a.OnCommunicationUp != nil {
		a.OnCommunicationUp(CommunicationUpEvent{})
	}
}

// HandleInbound parses incoming raw packets
func (a *Association) HandleInbound(raw []byte) error {
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

func (a *Association) packetizeOutbound(raw []byte, streamIdentifier uint16, payloadType PayloadProtocolID) ([]*chunkPayloadData, error) {

	if len(raw) > math.MaxUint16 {
		return nil, errors.Errorf("Outbound packet larger than maximum message size %v", math.MaxUint16)
	}

	seqNum, ok := a.outboundStreams[streamIdentifier]

	if !ok {
		seqNum = 0
	}

	i := uint16(0)
	remaining := uint16(len(raw))

	var chunks []*chunkPayloadData
	for remaining != 0 {
		l := min(a.myMaxMTU, remaining)
		chunks = append(chunks, &chunkPayloadData{
			streamID:             streamIdentifier,
			userData:             raw[i : i+l],
			beginingFragment:     i == 0,
			endingFragment:       remaining-l == 0,
			immediateSack:        false,
			payloadProtocolID:    payloadType,
			streamSequenceNumber: seqNum,
			tsn:                  a.myNextTSN,
		})
		a.myNextTSN++
		remaining -= l
		i += l
	}

	a.outboundStreams[streamIdentifier] = seqNum + 1

	return chunks, nil
}

// Send is the main method to send user data via SCTP.
func (a *Association) Send(raw []byte, streamIdentifier uint16, payloadType PayloadProtocolID) error {
	chunks, err := a.packetizeOutbound(raw, streamIdentifier, payloadType)
	if err != nil {
		return errors.Wrap(err, "Unable to packetize writer packet")
	}

	for _, c := range chunks {
		// TODO: FIX THIS HACK, inflightQueue uses PayloadQueue which is really meant for reader SACK generation
		a.inflightQueue.pushNoCheck(c)

		p := &packet{
			sourcePort:      a.sourcePort,
			destinationPort: a.destinationPort,
			verificationTag: a.peerVerificationTag,
			chunks:          []chunk{c}}
		if err := a.send(p); err != nil {
			return errors.Wrap(err, "Unable to send writer packet")
		}

	}
	return nil
}

// Close ends the SCTP Association and cleans up any state
func (a *Association) Close() error {
	return nil
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

func (a *Association) handleInit(p *packet, i *chunkInit) *packet {

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

	return outbound
}

func (a *Association) handleData(d *chunkPayloadData) *packet {

	a.payloadQueue.push(d, a.peerLastTSN)

	pd, popOk := a.payloadQueue.pop(a.peerLastTSN + 1)

	for popOk {
		rq, ok := a.reassemblyQueue[pd.streamID]
		if !ok {
			// If this is the first time we've seen a stream identifier
			// Expected SeqNum == 0
			rq = &reassemblyQueue{}
			a.reassemblyQueue[pd.streamID] = rq
		}

		rq.push(pd)
		userData, ok := rq.pop()
		if ok {
			// We know the popped data will have the same stream
			// identifier as the pushed data
			if a.OnReceive != nil {
				a.OnReceive(ReceiveEvent{
					Buffer:            userData,
					StreamID:          pd.streamID,
					PayloadProtocolID: pd.payloadProtocolID,
				})
			}
		}

		a.peerLastTSN++
		pd, popOk = a.payloadQueue.pop(a.peerLastTSN)
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

	return outbound
}

func (a *Association) handleSack(d *chunkSelectiveAck) ([]*packet, error) {
	// i) If Cumulative TSN Ack is less than the Cumulative TSN Ack
	// Point, then drop the SACK.  Since Cumulative TSN Ack is
	// monotonically increasing, a SACK whose Cumulative TSN Ack is
	// less than the Cumulative TSN Ack Point indicates an out-of-
	// order SACK.
	if a.firstSack {
		a.firstSack = false
		// We need the ack point to be 1 less than the real one so that when we get our first CumTSN
		// we act like it is an "new" ack point
		a.peerCumulativeTSNAckPoint = d.cumulativeTSNAck - 1
	}

	// This is an old SACK, toss
	if a.peerCumulativeTSNAckPoint >= d.cumulativeTSNAck {
		return nil, errors.Errorf("SACK Cumulative ACK %v is older than ACK point %v",
			d.cumulativeTSNAck, a.peerCumulativeTSNAckPoint)
	}

	// New ack point, so pop all ACKed packets from inflightQueue
	// We add 1 because the "currentAckPoint" has already been popped from the inflight queue
	// For the first SACK we take care of this by setting the ackpoint to cumAck - 1
	for i := a.peerCumulativeTSNAckPoint + 1; i <= d.cumulativeTSNAck; i++ {
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
			pp, ok := a.inflightQueue.get(d.cumulativeTSNAck + uint32(i))
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
		return errors.Wrap(err, "Failed to send packet to writer handler")
	}

	a.writer <- raw
	return nil
}

func (a *Association) handleChunk(p *packet, c chunk) error {
	if _, err := c.check(); err != nil {
		return errors.Wrap(err, "Failed validating chunk")
		// TODO: Create ABORT
	}

	switch c := c.(type) {
	case *chunkInit:
		switch a.state {
		case Open:
			return a.send(a.handleInit(p, c))
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
	case *chunkInitAck:

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
		return a.send(a.handleData(c))
	case *chunkSelectiveAck:
		p, err := a.handleSack(c)
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
