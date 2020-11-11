// +build !js

package webrtc

import (
	"sync"

	"github.com/pion/rtp"
)

// TrackRemote represents a single inbound source of media
type TrackRemote struct {
	mu sync.RWMutex

	id       string
	streamID string

	payloadType PayloadType
	kind        RTPCodecType
	ssrc        SSRC
	codec       RTPCodecParameters
	rid         string

	receiver *RTPReceiver
	peeked   []byte
}

// ID is the unique identifier for this Track. This should be unique for the
// stream, but doesn't have to globally unique. A common example would be 'audio' or 'video'
// and StreamID would be 'desktop' or 'webcam'
func (t *TrackRemote) ID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.id
}

// RID gets the RTP Stream ID of this Track
// With Simulcast you will have multiple tracks with the same ID, but different RID values.
// In many cases a TrackRemote will not have an RID, so it is important to assert it is non-zero
func (t *TrackRemote) RID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.rid
}

// PayloadType gets the PayloadType of the track
func (t *TrackRemote) PayloadType() PayloadType {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.payloadType
}

// Kind gets the Kind of the track
func (t *TrackRemote) Kind() RTPCodecType {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.kind
}

// StreamID is the group this track belongs too. This must be unique
func (t *TrackRemote) StreamID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.streamID
}

// SSRC gets the SSRC of the track
func (t *TrackRemote) SSRC() SSRC {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ssrc
}

// Msid gets the Msid of the track
func (t *TrackRemote) Msid() string {
	return t.StreamID() + " " + t.ID()
}

// Codec gets the Codec of the track
func (t *TrackRemote) Codec() RTPCodecParameters {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.codec
}

// Read reads data from the track.
func (t *TrackRemote) Read(b []byte) (n int, err error) {
	t.mu.RLock()
	r := t.receiver
	peeked := t.peeked != nil
	t.mu.RUnlock()

	if peeked {
		t.mu.Lock()
		data := t.peeked
		t.peeked = nil
		t.mu.Unlock()
		// someone else may have stolen our packet when we
		// released the lock.  Deal with it.
		if data != nil {
			n = copy(b, data)
			return
		}
	}

	return r.readRTP(b, t)
}

// peek is like Read, but it doesn't discard the packet read
func (t *TrackRemote) peek(b []byte) (n int, err error) {
	n, err = t.Read(b)
	if err != nil {
		return
	}

	t.mu.Lock()
	// this might overwrite data if somebody peeked between the Read
	// and us getting the lock.  Oh well, we'll just drop a packet in
	// that case.
	data := make([]byte, n)
	n = copy(data, b[:n])
	t.peeked = data
	t.mu.Unlock()
	return
}

// ReadRTP is a convenience method that wraps Read and unmarshals for you
func (t *TrackRemote) ReadRTP() (*rtp.Packet, error) {
	b := make([]byte, receiveMTU)
	i, err := t.Read(b)
	if err != nil {
		return nil, err
	}

	r := &rtp.Packet{}
	if err := r.Unmarshal(b[:i]); err != nil {
		return nil, err
	}
	return r, nil
}

// determinePayloadType blocks and reads a single packet to determine the PayloadType for this Track
// this is useful because we can't announce it to the user until we know the payloadType
func (t *TrackRemote) determinePayloadType() error {
	b := make([]byte, receiveMTU)
	n, err := t.peek(b)
	if err != nil {
		return err
	}
	r := rtp.Packet{}
	if err := r.Unmarshal(b[:n]); err != nil {
		return err
	}

	t.mu.Lock()
	t.payloadType = PayloadType(r.PayloadType)
	defer t.mu.Unlock()

	return nil
}
