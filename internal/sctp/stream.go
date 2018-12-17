package sctp

import (
	"math"
	"sync"

	"github.com/pkg/errors"
)

// Stream represents an SCTP stream
type Stream struct {
	association *Association

	lock sync.RWMutex

	streamIdentifier   uint16
	defaultPayloadType PayloadProtocolIdentifier

	reassemblyQueue *reassemblyQueue
	sequenceNumber  uint16

	readNotifier chan struct{}
	closeCh      chan struct{}
}

// StreamIdentifier returns the Stream identifier associated to the stream.
func (s *Stream) StreamIdentifier() uint16 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.streamIdentifier
}

// SetDefaultPayloadType sets the default payload type used by Write.
func (s *Stream) SetDefaultPayloadType(defaultPayloadType PayloadProtocolIdentifier) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.setDefaultPayloadType(defaultPayloadType)
}

// setDefaultPayloadType sets the defaultPayloadType. The caller should hold the lock.
func (s *Stream) setDefaultPayloadType(defaultPayloadType PayloadProtocolIdentifier) {
	s.defaultPayloadType = defaultPayloadType
}

// Read reads a packet of len(p) bytes, dropping the Payload Protocol Identifier
func (s *Stream) Read(p []byte) (int, error) {
	n, _, err := s.ReadSCTP(p)
	return n, err
}

// ReadSCTP reads a packet of len(p) bytes and returns the associated Payload Protocol Identifier
func (s *Stream) ReadSCTP(p []byte) (int, PayloadProtocolIdentifier, error) {
	for range s.readNotifier {
		s.lock.Lock()
		userData, ppi, ok := s.reassemblyQueue.pop() // TODO: pop into p?
		s.lock.Unlock()
		if ok {
			n := copy(p, userData)
			// TODO: check small buffer
			return n, ppi, nil
		}
	}

	return 0, PayloadProtocolIdentifier(0), errors.New("stream closed")
}

func (s *Stream) handleData(pd *chunkPayloadData) {
	s.lock.Lock()
	s.reassemblyQueue.push(pd)
	s.lock.Unlock()

	// Notify the reader
	select {
	case s.readNotifier <- struct{}{}:
	case <-s.closeCh:
	}
}

// Write writes len(p) bytes from p with the default Payload Protocol Identifier
func (s *Stream) Write(p []byte) (n int, err error) {
	return s.WriteSCTP(p, s.defaultPayloadType)
}

// WriteSCTP writes len(p) bytes from p to the DTLS connection
func (s *Stream) WriteSCTP(p []byte, ppi PayloadProtocolIdentifier) (n int, err error) {
	if len(p) > math.MaxUint16 {
		return 0, errors.Errorf("Outbound packet larger than maximum message size %v", math.MaxUint16)
	}

	chunks := s.packetize(p, ppi)

	return len(p), s.association.sendPayloadData(chunks)
}

func (s *Stream) packetize(raw []byte, ppi PayloadProtocolIdentifier) []*chunkPayloadData {
	s.lock.Lock()
	defer s.lock.Unlock()

	i := uint16(0)
	remaining := uint16(len(raw))

	var chunks []*chunkPayloadData
	for remaining != 0 {
		l := min(s.association.myMaxMTU, remaining)
		chunks = append(chunks, &chunkPayloadData{
			streamIdentifier:     s.streamIdentifier,
			userData:             raw[i : i+l],
			beginingFragment:     i == 0,
			endingFragment:       remaining-l == 0,
			immediateSack:        false,
			payloadType:          ppi,
			streamSequenceNumber: s.sequenceNumber,
		})
		remaining -= l
		i += l
	}

	s.sequenceNumber++

	return chunks
}

// Close closes the conn and releases any Read calls
func (s *Stream) Close() error {
	s.unregister()

	// TODO: reset stream?
	// https://tools.ietf.org/html/rfc6525

	return nil
}

func (s *Stream) unregister() {
	a := s.association
	close(s.closeCh)
	a.lock.Lock()
	defer a.lock.Unlock()
	close(s.readNotifier)
	delete(a.streams, s.streamIdentifier)
}
