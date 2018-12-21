package sctp

import (
	"bytes"
	"fmt"
	"net"
	"sync"

	"math"
	"math/rand"
	"time"

	"github.com/pkg/errors"
)

const receiveMTU = 8192

var errAssociationClosed = errors.New("The association is closed")

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
	lock sync.Mutex

	nextConn net.Conn

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
	peerCumulativeTSNAckPoint uint32

	streams              map[uint16]*Stream
	acceptCh             chan *Stream
	doneCh               chan struct{}
	handshakeCompletedCh chan struct{}
}

// Server accepts a SCTP stream over a conn
func Server(nextConn net.Conn) (*Association, error) {
	a := createAssocation(nextConn)
	go a.readLoop()
	<-a.handshakeCompletedCh

	return a, nil
}

// Client opens a SCTP stream over a conn
func Client(nextConn net.Conn) (*Association, error) {
	a := createAssocation(nextConn)
	go a.readLoop()
	a.init()
	<-a.handshakeCompletedCh

	return a, nil
}

func createAssocation(nextConn net.Conn) *Association {
	rs := rand.NewSource(time.Now().UnixNano())
	r := rand.New(rs)

	tsn := r.Uint32()
	return &Association{
		nextConn:                  nextConn,
		myMaxNumOutboundStreams:   math.MaxUint16,
		myMaxNumInboundStreams:    math.MaxUint16,
		myReceiverWindowCredit:    10 * 1500, // 10 Max MTU packets buffer
		payloadQueue:              &payloadQueue{},
		inflightQueue:             &payloadQueue{},
		myMaxMTU:                  1200,
		myVerificationTag:         r.Uint32(),
		myNextTSN:                 tsn,
		state:                     Open,
		streams:                   make(map[uint16]*Stream),
		acceptCh:                  make(chan *Stream),
		doneCh:                    make(chan struct{}),
		handshakeCompletedCh:      make(chan struct{}),
		peerCumulativeTSNAckPoint: tsn - 1,
	}
}

func (a *Association) init() {
	a.lock.Lock()
	defer a.lock.Unlock()

	err := a.send(a.createInit())
	if err != nil {
		fmt.Printf("Failed to send init: %v", err)
	}
	a.setState(CookieWait)
}

// Close ends the SCTP Association and cleans up any state
func (a *Association) Close() error {
	err := a.nextConn.Close()
	if err != nil {
		return err
	}

	// Wait for readLoop to end
	<-a.doneCh

	return nil
}

func (a *Association) readLoop() {
	defer func() {
		a.lock.Lock()
		for _, s := range a.streams {
			close(s.closeCh)
			close(s.readNotifier)
			delete(a.streams, s.streamIdentifier)
		}
		a.lock.Unlock()
		close(a.acceptCh)
		close(a.doneCh)
	}()
	for {
		// buffer is recreated because the user data is
		// passed to the reassembly queue without copying
		buffer := make([]byte, receiveMTU)
		n, err := a.nextConn.Read(buffer)
		if err != nil {
			return
		}

		if err = a.handleInbound(buffer[:n]); err != nil {
			fmt.Println(errors.Wrap(err, "Failed to push SCTP packet"))
		}
	}
}

// HandleInbound parses incoming raw packets
func (a *Association) handleInbound(raw []byte) error {
	p := &packet{}
	if err := p.unmarshal(raw); err != nil {
		return errors.Wrap(err, "Unable to parse SCTP packet")
	}

	if err := checkPacket(p); err != nil {
		return errors.Wrap(err, "Failed validating packet")
	}

	for _, c := range p.chunks {
		packets, err := a.handleChunk(p, c)
		if err != nil {
			return errors.Wrap(err, "Failed handling chunk")
		}
		for _, p := range packets {
			err = a.send(p)
			if err != nil {
				return errors.Wrap(err, "Failed sending reply")
			}
		}
	}

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

// setState sets the state of the Association.
func (a *Association) setState(state AssociationState) {
	if a.state != state {
		a.state = state
	}
}

// The caller should hold the lock.
func (a *Association) createInit() *packet {
	outbound := &packet{}
	outbound.verificationTag = a.peerVerificationTag
	a.sourcePort = 5000      // TODO: Spec??
	a.destinationPort = 5000 // TODO: Spec??
	outbound.sourcePort = a.sourcePort
	outbound.destinationPort = a.destinationPort

	init := &chunkInit{}

	init.initialTSN = a.myNextTSN
	init.numOutboundStreams = a.myMaxNumOutboundStreams
	init.numInboundStreams = a.myMaxNumInboundStreams
	init.initiateTag = a.myVerificationTag
	init.advertisedReceiverWindowCredit = a.myReceiverWindowCredit

	outbound.chunks = []chunk{init}

	return outbound
}

// The caller should hold the lock.
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

// The caller should hold the lock.
func (a *Association) handleInitAck(p *packet, i *chunkInitAck) (*packet, error) {
	a.myMaxNumInboundStreams = min(i.numInboundStreams, a.myMaxNumInboundStreams)
	a.myMaxNumOutboundStreams = min(i.numOutboundStreams, a.myMaxNumOutboundStreams)
	a.peerVerificationTag = i.initiateTag
	a.peerLastTSN = i.initialTSN - 1
	if a.sourcePort != p.destinationPort ||
		a.destinationPort != p.sourcePort {
		fmt.Println("handleInitAck: port mismatch")
	}

	outbound := &packet{}
	outbound.verificationTag = a.peerVerificationTag
	outbound.sourcePort = a.sourcePort
	outbound.destinationPort = a.destinationPort

	var cookieParam *paramStateCookie
	for _, param := range i.params {
		switch v := param.(type) {
		case *paramStateCookie:
			cookieParam = v
		}
	}
	if cookieParam == nil {
		return nil, errors.New("no cookie in InitAck")
	}

	cookieEcho := &chunkCookieEcho{}

	cookieEcho.cookie = cookieParam.cookie

	outbound.chunks = []chunk{cookieEcho}

	return outbound, nil
}

// The caller should hold the lock.
func (a *Association) handleData(d *chunkPayloadData) *packet {
	a.payloadQueue.push(d, a.peerLastTSN)

	pd, popOk := a.payloadQueue.pop(a.peerLastTSN + 1)

	for popOk {
		s := a.getOrCreateStream(pd.streamIdentifier)
		s.handleData(pd)

		a.peerLastTSN++
		pd, popOk = a.payloadQueue.pop(a.peerLastTSN + 1)
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

// OpenStream opens a stream
func (a *Association) OpenStream(streamIdentifier uint16, defaultPayloadType PayloadProtocolIdentifier) (*Stream, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	if _, ok := a.streams[streamIdentifier]; ok {
		return nil, fmt.Errorf("there already exists a stream with identifier %d", streamIdentifier)
	}

	s := a.createStream(streamIdentifier, false)
	s.setDefaultPayloadType(defaultPayloadType)

	return s, nil
}

// AcceptStream accepts a stream
func (a *Association) AcceptStream() (*Stream, error) {
	s, ok := <-a.acceptCh
	if !ok {
		return nil, errAssociationClosed
	}
	return s, nil
}

// createStream creates a stream. The caller should hold the lock and check no stream exists for this id.
func (a *Association) createStream(streamIdentifier uint16, accept bool) *Stream {
	s := &Stream{
		association:      a,
		streamIdentifier: streamIdentifier,
		reassemblyQueue:  &reassemblyQueue{},
		readNotifier:     make(chan struct{}),
		closeCh:          make(chan struct{}),
	}

	a.streams[streamIdentifier] = s

	if accept {
		a.acceptCh <- s
	}

	return s
}

// getOrCreateStream gets or creates a stream. The caller should hold the lock.
func (a *Association) getOrCreateStream(streamIdentifier uint16) *Stream {
	if s, ok := a.streams[streamIdentifier]; ok {
		return s
	}

	return a.createStream(streamIdentifier, true)
}

// The caller should hold the lock.
func (a *Association) handleSack(d *chunkSelectiveAck) ([]*packet, error) {
	// i) If Cumulative TSN Ack is less than the Cumulative TSN Ack
	// Point, then drop the SACK.  Since Cumulative TSN Ack is
	// monotonically increasing, a SACK whose Cumulative TSN Ack is
	// less than the Cumulative TSN Ack Point indicates an out-of-
	// order SACK.

	// This is an old SACK, toss
	if a.peerCumulativeTSNAckPoint > d.cumulativeTSNAck {
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

// sendPayloadData sends the data chunks.
func (a *Association) sendPayloadData(chunks []*chunkPayloadData) error {
	packets := []*packet{}

	a.lock.Lock()
	for _, c := range chunks {
		c.tsn = a.generateNextTSN()

		// TODO: FIX THIS HACK, inflightQueue uses PayloadQueue which is really meant for inbound SACK generation
		a.inflightQueue.pushNoCheck(c)

		p := &packet{
			sourcePort:      a.sourcePort,
			destinationPort: a.destinationPort,
			verificationTag: a.peerVerificationTag,
			chunks:          []chunk{c}}
		packets = append(packets, p)
	}
	a.lock.Unlock()

	for _, p := range packets {
		if err := a.send(p); err != nil {
			return errors.Wrap(err, "Unable to send outbound packet")
		}
	}

	return nil
}

// generateNextTSN returns the myNextTSN and increases it. The caller should hold the lock.
func (a *Association) generateNextTSN() uint32 {
	tsn := a.myNextTSN
	a.myNextTSN++
	return tsn
}

// send sends a packet over nextConn. The caller should hold the lock.
func (a *Association) send(p *packet) error {
	raw, err := p.marshal()
	if err != nil {
		return errors.Wrap(err, "Failed to send packet to outbound handler")
	}

	_, err = a.nextConn.Write(raw)
	return err
}

func pack(p *packet) []*packet {
	return []*packet{p}
}

func (a *Association) handleChunk(p *packet, c chunk) ([]*packet, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	if _, err := c.check(); err != nil {
		return nil, errors.Wrap(err, "Failed validating chunk")
		// TODO: Create ABORT
	}

	switch c := c.(type) {
	case *chunkInit:
		switch a.state {
		case Open:
			return pack(a.handleInit(p, c)), nil
		case CookieWait:
			// https://tools.ietf.org/html/rfc4960#section-5.2.1
			// Upon receipt of an INIT in the COOKIE-WAIT state, an endpoint MUST
			// respond with an INIT ACK using the same parameters it sent in its
			// original INIT chunk (including its Initiate Tag, unchanged).  When
			// responding, the endpoint MUST send the INIT ACK back to the same
			// address that the original INIT (sent by this endpoint) was sent.
			return pack(a.handleInit(p, c)), nil

		case CookieEchoed:
			// https://tools.ietf.org/html/rfc4960#section-5.2.1
			// Upon receipt of an INIT in the COOKIE-ECHOED state, an endpoint MUST
			// respond with an INIT ACK using the same parameters it sent in its
			// original INIT chunk (including its Initiate Tag, unchanged)
			return nil, errors.Errorf("TODO respond with original cookie %s", a.state.String())
		default:
			// 5.2.2.  Unexpected INIT in States Other than CLOSED, COOKIE-ECHOED,
			//        COOKIE-WAIT, and SHUTDOWN-ACK-SENT
			return nil, errors.Errorf("TODO Handle Init when in state %s", a.state.String())
		}

	case *chunkInitAck:
		switch a.state {
		case CookieWait:
			r, err := a.handleInitAck(p, c)
			if err != nil {
				return nil, err
			}
			a.setState(CookieEchoed)
			return pack(r), nil
		default:
			return nil, errors.Errorf("TODO Handle Init acks when in state %s", a.state.String())
		}

	case *chunkAbort:
		fmt.Println("Abort chunk, with errors:")
		for _, e := range c.errorCauses {
			fmt.Printf("error cause: %s\n", e)
		}

	case *chunkHeartbeat:
		hbi, ok := c.params[0].(*paramHeartbeatInfo)
		if !ok {
			fmt.Println("Failed to handle Heartbeat, no ParamHeartbeatInfo")
		}

		return pack(&packet{
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
		}), nil

	case *chunkCookieEcho:
		if bytes.Equal(a.myCookie.cookie, c.cookie) {
			p := &packet{
				verificationTag: a.peerVerificationTag,
				sourcePort:      a.sourcePort,
				destinationPort: a.destinationPort,
				chunks:          []chunk{&chunkCookieAck{}},
			}
			a.setState(Established)
			close(a.handshakeCompletedCh)

			return pack(p), nil
		}

	case *chunkCookieAck:
		switch a.state {
		case CookieEchoed:
			a.setState(Established)
			close(a.handshakeCompletedCh)
			return nil, nil
		default:
			return nil, errors.Errorf("TODO Handle Init acks when in state %s", a.state.String())
		}

		// TODO Abort
	case *chunkPayloadData:
		return pack(a.handleData(c)), nil

	case *chunkSelectiveAck:
		p, err := a.handleSack(c)
		if err != nil {
			return nil, errors.Wrap(err, "Failure handling SACK")
		}
		return p, nil

	default:
		return nil, errors.New("unhandled chunk type")
	}

	return nil, nil
}
