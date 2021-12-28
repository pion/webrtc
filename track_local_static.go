// +build !js

package webrtc

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/internal/util"
	"github.com/pion/webrtc/v3/pkg/media"
	log "github.com/sirupsen/logrus"
)

// trackBinding is a single bind for a Track
// Bind can be called multiple times, this stores the
// result for a single bind call so that it can be used when writing
type trackBinding struct {
	id          string
	ssrc        SSRC
	payloadType PayloadType
	writeStream TrackLocalWriter
}

// TrackLocalStaticRTP  is a TrackLocal that has a pre-set codec and accepts RTP Packets.
// If you wish to send a media.Sample use TrackLocalStaticSample
type TrackLocalStaticRTP struct {
	mu           sync.RWMutex
	bindings     []trackBinding
	codec        RTPCodecCapability
	id, streamID string
}

// NewTrackLocalStaticRTP returns a TrackLocalStaticRTP.
func NewTrackLocalStaticRTP(c RTPCodecCapability, id, streamID string) (*TrackLocalStaticRTP, error) {
	return &TrackLocalStaticRTP{
		codec:    c,
		bindings: []trackBinding{},
		id:       id,
		streamID: streamID,
	}, nil
}

// Bind is called by the PeerConnection after negotiation is complete
// This asserts that the code requested is supported by the remote peer.
// If so it setups all the state (SSRC and PayloadType) to have a call
func (s *TrackLocalStaticRTP) Bind(t TrackLocalContext) (RTPCodecParameters, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parameters := RTPCodecParameters{RTPCodecCapability: s.codec}
	if codec, matchType := codecParametersFuzzySearch(parameters, t.CodecParameters()); matchType != codecMatchNone {
		s.bindings = append(s.bindings, trackBinding{
			ssrc:        t.SSRC(),
			payloadType: codec.PayloadType,
			writeStream: t.WriteStream(),
			id:          t.ID(),
		})
		return codec, nil
	}

	return RTPCodecParameters{}, ErrUnsupportedCodec
}

// Unbind implements the teardown logic when the track is no longer needed. This happens
// because a track has been stopped.
func (s *TrackLocalStaticRTP) Unbind(t TrackLocalContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.bindings {
		if s.bindings[i].id == t.ID() {
			s.bindings[i] = s.bindings[len(s.bindings)-1]
			s.bindings = s.bindings[:len(s.bindings)-1]
			return nil
		}
	}

	return ErrUnbindFailed
}

// ID is the unique identifier for this Track. This should be unique for the
// stream, but doesn't have to globally unique. A common example would be 'audio' or 'video'
// and StreamID would be 'desktop' or 'webcam'
func (s *TrackLocalStaticRTP) ID() string { return s.id }

// StreamID is the group this track belongs too. This must be unique
func (s *TrackLocalStaticRTP) StreamID() string { return s.streamID }

// Kind controls if this TrackLocal is audio or video
func (s *TrackLocalStaticRTP) Kind() RTPCodecType {
	switch {
	case strings.HasPrefix(s.codec.MimeType, "audio/"):
		return RTPCodecTypeAudio
	case strings.HasPrefix(s.codec.MimeType, "video/"):
		return RTPCodecTypeVideo
	default:
		return RTPCodecType(0)
	}
}

// Codec gets the Codec of the track
func (s *TrackLocalStaticRTP) Codec() RTPCodecCapability {
	return s.codec
}

// packetPool is a pool of packets used by WriteRTP and Write below
// nolint:gochecknoglobals
var rtpPacketPool = sync.Pool{
	New: func() interface{} {
		return &rtp.Packet{}
	},
}

// WriteRTP writes a RTP Packet to the TrackLocalStaticRTP
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them
func (s *TrackLocalStaticRTP) WriteRTP(p *rtp.Packet) error {
	ipacket := rtpPacketPool.Get()
	packet := ipacket.(*rtp.Packet)
	defer func() {
		*packet = rtp.Packet{}
		rtpPacketPool.Put(ipacket)
	}()
	*packet = *p
	return s.writeRTP(packet)
}

// writeRTP is like WriteRTP, except that it may modify the packet p
func (s *TrackLocalStaticRTP) writeRTP(p *rtp.Packet) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	writeErrs := []error{}

	for _, b := range s.bindings {
		p.Header.SSRC = uint32(b.ssrc)
		p.Header.PayloadType = uint8(b.payloadType)
		log.WithFields(
			log.Fields{
				"type":           "INTENSIVE",
				"subcomponent":   "webrtc",
				"ssrc":           p.Header.SSRC,
				"timestamp":      p.Timestamp,
				"sequenceNumber": p.SequenceNumber,
				"hasExtension":   p.Extension,
				"extensions":     fmt.Sprintf("%v", p.Extensions),
			}).Trace("outgoing rtp..")
		if _, err := b.writeStream.WriteRTP(&p.Header, p.Payload); err != nil {
			writeErrs = append(writeErrs, err)
		}
	}

	return util.FlattenErrs(writeErrs)
}

// Write writes a RTP Packet as a buffer to the TrackLocalStaticRTP
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them
func (s *TrackLocalStaticRTP) Write(b []byte) (n int, err error) {
	ipacket := rtpPacketPool.Get()
	packet := ipacket.(*rtp.Packet)
	defer func() {
		*packet = rtp.Packet{}
		rtpPacketPool.Put(ipacket)
	}()

	if err = packet.Unmarshal(b); err != nil {
		return 0, err
	}

	return len(b), s.writeRTP(packet)
}

// TrackLocalStaticSample is a TrackLocal that has a pre-set codec and accepts Samples.
// If you wish to send a RTP Packet use TrackLocalStaticRTP
type TrackLocalStaticSample struct {
	Packetizer rtp.Packetizer
	sequencer  rtp.Sequencer
	RtpTrack   *TrackLocalStaticRTP
	ClockRate  float64
}

// NewTrackLocalStaticSample returns a TrackLocalStaticSample
func NewTrackLocalStaticSample(c RTPCodecCapability, id, streamID string) (*TrackLocalStaticSample, error) {
	rtpTrack, err := NewTrackLocalStaticRTP(c, id, streamID)
	if err != nil {
		return nil, err
	}

	return &TrackLocalStaticSample{
		RtpTrack: rtpTrack,
	}, nil
}

// ID is the unique identifier for this Track. This should be unique for the
// stream, but doesn't have to globally unique. A common example would be 'audio' or 'video'
// and StreamID would be 'desktop' or 'webcam'
func (s *TrackLocalStaticSample) ID() string { return s.RtpTrack.ID() }

// StreamID is the group this track belongs too. This must be unique
func (s *TrackLocalStaticSample) StreamID() string { return s.RtpTrack.StreamID() }

// Kind controls if this TrackLocal is audio or video
func (s *TrackLocalStaticSample) Kind() RTPCodecType { return s.RtpTrack.Kind() }

// Codec gets the Codec of the track
func (s *TrackLocalStaticSample) Codec() RTPCodecCapability {
	return s.RtpTrack.Codec()
}

// Bind is called by the PeerConnection after negotiation is complete
// This asserts that the code requested is supported by the remote peer.
// If so it setups all the state (SSRC and PayloadType) to have a call
func (s *TrackLocalStaticSample) Bind(t TrackLocalContext) (RTPCodecParameters, error) {
	codec, err := s.RtpTrack.Bind(t)
	if err != nil {
		return codec, err
	}

	s.RtpTrack.mu.Lock()
	defer s.RtpTrack.mu.Unlock()

	// We only need one Packetizer
	if s.Packetizer != nil {
		return codec, nil
	}

	payloader, err := payloaderForCodec(codec.RTPCodecCapability)
	if err != nil {
		return codec, err
	}

	s.sequencer = rtp.NewRandomSequencer()
	s.Packetizer = rtp.NewInterleavedPacketizer(
		getRtpOutboundMtu(),
		0, // Value is handled when writing
		0, // Value is handled when writing
		payloader,
		s.sequencer,
		codec.ClockRate,
	)
	s.ClockRate = float64(codec.RTPCodecCapability.ClockRate)
	return codec, nil
}

// Unbind implements the teardown logic when the track is no longer needed. This happens
// because a track has been stopped.
func (s *TrackLocalStaticSample) Unbind(t TrackLocalContext) error {
	return s.RtpTrack.Unbind(t)
}

// WriteSample writes a Sample to the TrackLocalStaticSample
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them
func (s *TrackLocalStaticSample) WriteSample(sample media.Sample, onRtpPacket func(*rtp.Packet)) error {
	s.RtpTrack.mu.RLock()
	p := s.Packetizer
	clockRate := s.ClockRate
	s.RtpTrack.mu.RUnlock()

	if p == nil {
		return nil
	}

	// skip packets by the number of previously dropped packets
	for i := uint16(0); i < sample.PrevDroppedPackets; i++ {
		s.sequencer.NextSequenceNumber()
	}

	samples := uint32(sample.Duration.Seconds() * clockRate)
	if sample.PrevDroppedPackets > 0 {
		p.(rtp.Packetizer).SkipSamples(samples * uint32(sample.PrevDroppedPackets))
	}
	packets := p.(rtp.Packetizer).Packetize(sample.Data, samples)

	err := addExtensions(sample, packets)

	if err != nil {
		log.WithFields(
			log.Fields{
				"subcomponent": "webrtc",
				"type":         "INTENSIVE",
				"err":          err.Error(),
				"hasExtension": packets[0].Extension,
				"extensions":   fmt.Sprintf("%v", packets[0].Extensions),
			}).Error("encountered an error when adding extension")
	}

	writeErrs := []error{}
	for _, p := range packets {
		if err := s.RtpTrack.WriteRTP(p); err != nil {
			writeErrs = append(writeErrs, err)
		}
		if onRtpPacket != nil {
			onRtpPacket(p)
		}
	}

	return util.FlattenErrs(writeErrs)
}

// WriteSample writes a Sample to the TrackLocalStaticSample
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them
func (s *TrackLocalStaticSample) WriteInterleavedSample(sample media.Sample, onRtpPacket func(*rtp.Packet)) error {
	s.RtpTrack.mu.RLock()
	p := s.Packetizer
	clockRate := s.ClockRate
	s.RtpTrack.mu.RUnlock()

	if p == nil {
		return nil
	}

	samples := sample.Duration.Seconds() * clockRate
	packets := p.(rtp.Packetizer).PacketizeInterleaved(sample.Data, uint32(samples))

	err := addExtensions(sample, packets)

	if err != nil {
		log.WithFields(
			log.Fields{
				"subcomponent": "webrtc",
				"type":         "INTENSIVE",
				"err":          err.Error(),
				"hasExtension": packets[0].Extension,
				"extensions":   fmt.Sprintf("%v", packets[0].Extensions),
			}).Error("encountered an error when adding extension")
	}

	writeErrs := []error{}
	for _, p := range packets {
		if err := s.RtpTrack.WriteRTP(p); err != nil {
			writeErrs = append(writeErrs, err)
		}
		if onRtpPacket != nil {
			onRtpPacket(p)
		}
	}

	return util.FlattenErrs(writeErrs)
}

func addExtensions(sample media.Sample, packets []*rtp.Packet) error {
	var sampleAttr byte = 0
	position, err := getExtensionVal("HYPERSCALE_RTP_EXTENSION_FIRST_PACKET_ATTR_POS")
	if err == nil {
		sampleAttr |= 1 << position
	}
	if position, err := getExtensionVal("HYPERSCALE_RTP_EXTENSION_IFRAME_ATTR_POS"); sample.IsIFrame && err == nil {
		sampleAttr |= 1 << position
	}
	if position, err := getExtensionVal("HYPERSCALE_RTP_EXTENSION_SPS_PPS_ATTR_POS"); sample.IsSpsPps && err == nil {
		sampleAttr |= 1 << position
	}
	if position, err := getExtensionVal("HYPERSCALE_RTP_EXTENSION_ABR_ATTR_POS"); sample.IsAbr && err == nil {
		sampleAttr |= 1 << position
	}

	extensionErrs := []error{}

	if len(packets) > 0 {
		extensionErrs = append(extensionErrs, packets[0].SetExtensions(sample.Extensions))
		if sample.WithHyperscaleExtensions {
			if id, err := getExtensionVal("HYPERSCALE_RTP_EXTENSION_SAMPLE_ATTR_ID"); err == nil {
				extensionErrs = append(extensionErrs, packets[0].SetExtension(id, []byte{sampleAttr}))
			}
			if id, err := getExtensionVal("HYPERSCALE_RTP_EXTENSION_DON_ID"); err == nil {
				donBytes := make([]byte, 2)
				binary.BigEndian.PutUint16(donBytes, sample.Don)
				extensionErrs = append(extensionErrs, packets[0].SetExtension(id, donBytes))
			}
		}
	}

	return util.FlattenErrs(extensionErrs)
}

func getExtensionVal(envVariable string) (uint8, error) {
	envValue := os.Getenv(envVariable)
	if envValue != "" {
		parsed, err := strconv.ParseUint(envValue, 10, 8)
		if err == nil {
			return uint8(parsed), nil
		}
		return 0, err
	}
	return 0, fmt.Errorf("extension value %s does not exist", envValue)
}

func getRtpOutboundMtu() uint16 {
	rtpOutboundMTUEnv := os.Getenv("HYPERSCALE_WEBRTC_RTP_OUTBOUND_MTU")
	if rtpOutboundMTUEnv != "" {
		parsed, err := strconv.ParseUint(rtpOutboundMTUEnv, 10, 16)
		if err == nil {
			return uint16(parsed)
		}
	}
	return rtpOutboundMTU
}
