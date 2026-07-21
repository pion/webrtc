// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
)

type peekedPacket struct {
	payload    []byte
	attributes interceptor.Attributes
}

type trackRemoteReadResult struct {
	payload    []byte
	attributes interceptor.Attributes
	err        error
}

type trackRemoteReadWorker struct {
	request  chan struct{}
	result   chan trackRemoteReadResult
	consumed chan struct{}
}

// TrackRemote represents a single inbound source of media.
type TrackRemote struct {
	readMu sync.Mutex
	mu     sync.RWMutex

	id       string
	streamID string

	payloadType PayloadType
	kind        RTPCodecType
	ssrc        SSRC
	rtxSsrc     SSRC
	codec       RTPCodecParameters
	params      RTPParameters
	rid         string

	receiver *RTPReceiver

	peekedPackets []*peekedPacket

	readWorker          *trackRemoteReadWorker
	primaryReadPending  bool
	primaryReadDetached bool

	audioPlayoutStatsProviders []AudioPlayoutStatsProvider
}

func newTrackRemote(kind RTPCodecType, ssrc, rtxSsrc SSRC, rid string, receiver *RTPReceiver) *TrackRemote {
	return &TrackRemote{
		kind:     kind,
		ssrc:     ssrc,
		rtxSsrc:  rtxSsrc,
		rid:      rid,
		receiver: receiver,
	}
}

// ID is the unique identifier for this Track. This should be unique for the
// stream, but doesn't have to globally unique. A common example would be 'audio' or 'video'
// and StreamID would be 'desktop' or 'webcam'.
func (t *TrackRemote) ID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.id
}

// RID gets the RTP Stream ID of this Track
// With Simulcast you will have multiple tracks with the same ID, but different RID values.
// In many cases a TrackRemote will not have an RID, so it is important to assert it is non-zero.
func (t *TrackRemote) RID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.rid
}

// PayloadType gets the PayloadType of the track.
func (t *TrackRemote) PayloadType() PayloadType {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.payloadType
}

// Kind gets the Kind of the track.
func (t *TrackRemote) Kind() RTPCodecType {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.kind
}

// StreamID is the group this track belongs too. This must be unique.
func (t *TrackRemote) StreamID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.streamID
}

// SSRC gets the SSRC of the track.
func (t *TrackRemote) SSRC() SSRC {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.ssrc
}

// Msid gets the Msid of the track.
func (t *TrackRemote) Msid() string {
	return t.StreamID() + " " + t.ID()
}

// Codec gets the Codec of the track.
func (t *TrackRemote) Codec() RTPCodecParameters {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.codec
}

// Read reads data from the track.
func (t *TrackRemote) Read(b []byte) (n int, attributes interceptor.Attributes, err error) {
	if !t.mayHaveRepairStream() {
		return t.readDirect(b)
	}

	t.readMu.Lock()
	defer t.readMu.Unlock()

	repairStreamChannel := t.receiver.requestRepairStreamReader(t)
	if repairStreamChannel == nil {
		return t.readDirect(b)
	}

	if peekedPkt := t.popPeekedPacket(); peekedPkt != nil {
		n = copy(b, peekedPkt.payload)

		return n, peekedPkt.attributes, t.checkAndUpdateTrack(b)
	}

	return t.readPrimaryOrRepair(b, repairStreamChannel)
}

func (t *TrackRemote) mayHaveRepairStream() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// An RSID repair stream may be negotiated and discovered after a RID track is exposed.
	return t.rtxSsrc != 0 || t.rid != ""
}

func (t *TrackRemote) popPeekedPacket() *peekedPacket {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.peekedPackets) == 0 {
		return nil
	}

	peekedPkt := t.peekedPackets[0]
	t.peekedPackets = t.peekedPackets[1:]

	return peekedPkt
}

func (t *TrackRemote) readDirect(b []byte) (n int, attributes interceptor.Attributes, err error) {
	receiver := t.receiver

	if receiver.haveClosed() {
		return 0, nil, io.EOF
	}

	if peekedPkt := t.popPeekedPacket(); peekedPkt != nil {
		n = copy(b, peekedPkt.payload)

		return n, peekedPkt.attributes, t.checkAndUpdateTrack(b)
	}

	// If there's a separate RTX track and an RTX packet is available, return that
	if packet := receiver.readRTX(t); packet != nil {
		return copyRTXPacket(b, packet)
	}

	n, attributes, err = receiver.readRTP(b, t)
	if err != nil {
		return n, attributes, err
	}
	err = t.checkAndUpdateTrack(b)

	return n, attributes, err
}

func copyRTXPacket(b []byte, packet *rtxPacketWithAttributes) (int, interceptor.Attributes, error) {
	packetLen := len(packet.pkt)
	n := copy(b, packet.pkt)
	attributes := packet.attributes
	packet.release()
	if n < packetLen {
		return n, attributes, io.ErrShortBuffer
	}

	return n, attributes, nil
}

func (t *TrackRemote) readPrimaryOrRepair(
	b []byte,
	repairStreamChannel chan rtxPacketWithAttributes,
) (int, interceptor.Attributes, error) {
	for {
		if packet := t.receiver.readRTXFromChannel(t, repairStreamChannel); packet != nil {
			t.primaryReadDetached = t.primaryReadPending

			return copyRTXPacket(b, packet)
		}

		if !t.primaryReadPending && !t.startPrimaryRead() {
			return 0, nil, io.EOF
		}

		select {
		case <-t.receiver.closedChan:
			return 0, nil, io.EOF
		case result := <-t.readWorker.result:
			discarded, n, attributes, err := t.consumePrimaryRead(b, result)
			if discarded {
				continue
			}

			return n, attributes, err
		case packet := <-repairStreamChannel:
			if !t.receiver.isCurrentRepairStream(t, packet.generation) {
				packet.release()

				continue
			}

			t.primaryReadDetached = true

			return copyRTXPacket(b, &packet)
		}
	}
}

func (t *TrackRemote) startPrimaryRead() bool {
	if t.readWorker == nil {
		t.readWorker = &trackRemoteReadWorker{
			request:  make(chan struct{}, 1),
			result:   make(chan trackRemoteReadResult),
			consumed: make(chan struct{}),
		}
		go t.runPrimaryReadWorker(t.readWorker)
	}

	select {
	case <-t.receiver.closedChan:
		return false
	case t.readWorker.request <- struct{}{}:
		t.primaryReadPending = true
		t.primaryReadDetached = false

		return true
	}
}

func (t *TrackRemote) runPrimaryReadWorker(worker *trackRemoteReadWorker) {
	mtu := uint(receiveMTU)
	if t.receiver.api != nil && t.receiver.api.settingEngine != nil {
		mtu = t.receiver.api.settingEngine.getReceiveMTU()
	}
	b := make([]byte, mtu)

	for {
		select {
		case <-t.receiver.closedChan:
			return
		case <-worker.request:
		}

		n, attributes, err := t.receiver.readRTP(b, t)
		select {
		case <-t.receiver.closedChan:
			return
		case worker.result <- trackRemoteReadResult{payload: b[:n], attributes: attributes, err: err}:
		}

		select {
		case <-t.receiver.closedChan:
			return
		case <-worker.consumed:
		}
	}
}

func (t *TrackRemote) consumePrimaryRead(
	b []byte,
	result trackRemoteReadResult,
) (discarded bool, n int, attributes interceptor.Attributes, err error) {
	detached := t.primaryReadDetached
	t.primaryReadPending = false
	t.primaryReadDetached = false

	attributes = result.attributes
	if !detached || result.err == nil {
		n = copy(b, result.payload)
	}
	select {
	case <-t.receiver.closedChan:
	case t.readWorker.consumed <- struct{}{}:
	}

	if detached && result.err != nil {
		return true, 0, nil, result.err
	}
	if result.err != nil {
		return false, n, attributes, result.err
	}
	if n < len(result.payload) {
		return false, n, attributes, io.ErrShortBuffer
	}

	return false, n, attributes, t.checkAndUpdateTrack(b)
}

// checkAndUpdateTrack checks payloadType for every incoming packet
// once a different payloadType is detected the track will be updated.
func (t *TrackRemote) checkAndUpdateTrack(b []byte) error {
	if len(b) < 2 {
		return errRTPTooShort
	}

	payloadType := PayloadType(b[1] & rtpPayloadTypeBitmask)
	if payloadType != t.PayloadType() || len(t.params.Codecs) == 0 {
		t.mu.Lock()
		defer t.mu.Unlock()

		params, err := t.receiver.api.mediaEngine.getRTPParametersByPayloadType(payloadType)
		if err != nil {
			return err
		}

		t.kind = t.receiver.kind
		t.payloadType = payloadType
		t.codec = params.Codecs[0]
		t.params = params
	}

	return nil
}

// ReadRTP is a convenience method that wraps Read and unmarshals for you.
func (t *TrackRemote) ReadRTP() (*rtp.Packet, interceptor.Attributes, error) {
	b := make([]byte, t.receiver.api.settingEngine.getReceiveMTU())
	i, attributes, err := t.Read(b)
	if err != nil {
		return nil, nil, err
	}

	r := &rtp.Packet{}
	if err := r.Unmarshal(b[:i]); err != nil {
		return nil, nil, err
	}

	return r, attributes, nil
}

// peek is like Read, but it doesn't discard the packet read.
func (t *TrackRemote) peek(b []byte) (int, error) {
	n, attributes, err := t.readDirect(b)
	if err != nil {
		return n, err
	}

	t.mu.Lock()
	// this might overwrite data if somebody peeked between the Read
	// and us getting the lock.  Oh well, we'll just drop a packet in
	// that case.
	data := make([]byte, n)
	n = copy(data, b[:n])
	t.peekedPackets = append(t.peekedPackets, &peekedPacket{payload: data, attributes: attributes})
	t.mu.Unlock()

	return n, nil
}

// SetReadDeadline sets the max amount of time the RTP stream will block before returning. 0 is forever.
func (t *TrackRemote) SetReadDeadline(deadline time.Time) error {
	return t.receiver.setRTPReadDeadline(deadline, t)
}

// RtxSSRC returns the RTX SSRC for a track, or 0 if track does not have a separate RTX stream.
func (t *TrackRemote) RtxSSRC() SSRC {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.rtxSsrc
}

// HasRTX returns true if the track has a separate RTX stream.
func (t *TrackRemote) HasRTX() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.rtxSsrc != 0
}

func (t *TrackRemote) addProvider(provider AudioPlayoutStatsProvider) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if slices.Contains(t.audioPlayoutStatsProviders, provider) {
		return
	}

	t.audioPlayoutStatsProviders = append(t.audioPlayoutStatsProviders, provider)
}

func (t *TrackRemote) removeProvider(provider AudioPlayoutStatsProvider) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, p := range t.audioPlayoutStatsProviders {
		if p == provider {
			t.audioPlayoutStatsProviders = append(t.audioPlayoutStatsProviders[:i], t.audioPlayoutStatsProviders[i+1:]...)

			return
		}
	}
}

func (t *TrackRemote) pullAudioPlayoutStats(now time.Time) []AudioPlayoutStats {
	t.mu.RLock()
	providers := t.audioPlayoutStatsProviders
	t.mu.RUnlock()

	if len(providers) == 0 {
		return nil
	}

	var allStats []AudioPlayoutStats
	for _, provider := range providers {
		stats, ok := provider.Snapshot(now)
		if !ok {
			continue
		}

		if stats.ID == "" {
			stats.ID = fmt.Sprintf("media-playout-%d", uint32(t.SSRC()))
		}

		if stats.Type == "" {
			stats.Type = StatsTypeMediaPlayout
		}

		if stats.Kind == "" {
			stats.Kind = string(MediaKindAudio)
		}

		if stats.Timestamp == 0 {
			stats.Timestamp = statsTimestampFrom(now)
		}

		allStats = append(allStats, stats)
	}

	return allStats
}

func (t *TrackRemote) setRtxSSRC(ssrc SSRC) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rtxSsrc = ssrc
}
