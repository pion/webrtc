// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/srtp/v3"
	"github.com/pion/webrtc/v4/internal/util"
)

// trackStreams maintains a mapping of RTP/RTCP streams to a specific track
// a RTPReceiver may contain multiple streams if we are dealing with Simulcast.
type trackStreams struct {
	track *TrackRemote

	streamInfo, repairStreamInfo *interceptor.StreamInfo

	rtpReadStream         *srtp.ReadStreamSRTP
	rtpInterceptor        interceptor.RTPReader
	rtpInterceptorWrapped bool

	rtcpReadStream  *srtp.ReadStreamSRTCP
	rtcpInterceptor interceptor.RTCPReader

	repairReadStream              *srtp.ReadStreamSRTP
	repairInterceptor             interceptor.RTPReader
	repairInterceptorWrapped      bool
	repairStreamChannel           chan rtxPacketWithAttributes
	repairStreamGeneration        uint64
	repairReaderStartedGeneration uint64
	publicRepairReadRequested     bool

	repairRtcpReadStream  *srtp.ReadStreamSRTCP
	repairRtcpInterceptor interceptor.RTCPReader
}

type rtxPacketWithAttributes struct {
	pkt        []byte
	attributes interceptor.Attributes
	pool       *sync.Pool
	generation uint64
}

func (p *rtxPacketWithAttributes) release() {
	if p.pkt != nil {
		b := p.pkt[:cap(p.pkt)]
		p.pool.Put(b) // nolint:staticcheck
		p.pkt = nil
	}
}

func (r *RTPReceiver) usesCustomBufferFactory() bool {
	return r.api != nil &&
		r.api.settingEngine != nil &&
		r.api.settingEngine.BufferFactory != nil
}

// RTPReceiver allows an application to inspect the receipt of a TrackRemote.
type RTPReceiver struct {
	kind      RTPCodecType
	transport *DTLSTransport

	tracks []trackStreams

	closed               atomic.Bool
	closedChan, received chan any
	mu                   sync.RWMutex

	tr *RTPTransceiver

	// A reference to the associated api object
	api *API

	rtxPool sync.Pool

	log logging.LeveledLogger
}

// NewRTPReceiver constructs a new RTPReceiver.
func (api *API) NewRTPReceiver(kind RTPCodecType, transport *DTLSTransport) (*RTPReceiver, error) {
	if transport == nil {
		return nil, errRTPReceiverDTLSTransportNil
	}

	rtpReceiver := &RTPReceiver{
		kind:       kind,
		transport:  transport,
		api:        api,
		closedChan: make(chan any),
		received:   make(chan any),
		tracks:     []trackStreams{},
		rtxPool: sync.Pool{New: func() any {
			return make([]byte, api.settingEngine.getReceiveMTU())
		}},
		log: api.settingEngine.LoggerFactory.NewLogger("RTPReceiver"),
	}

	return rtpReceiver, nil
}

func (r *RTPReceiver) setRTPTransceiver(tr *RTPTransceiver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tr = tr
}

// Transport returns the currently-configured *DTLSTransport or nil
// if one has not yet been configured.
func (r *RTPReceiver) Transport() *DTLSTransport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.transport
}

func (r *RTPReceiver) getParameters() RTPParameters {
	parameters := r.api.mediaEngine.getRTPParametersByKind(
		r.kind,
		[]RTPTransceiverDirection{RTPTransceiverDirectionRecvonly},
	)
	if r.tr != nil {
		parameters.Codecs = r.tr.getCodecs()
	}

	return parameters
}

// GetParameters describes the current configuration for the encoding and
// transmission of media on the receiver's track.
func (r *RTPReceiver) GetParameters() RTPParameters {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getParameters()
}

// Track returns the RtpTransceiver TrackRemote.
func (r *RTPReceiver) Track() *TrackRemote {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.tracks) != 1 {
		return nil
	}

	return r.tracks[0].track
}

// Tracks returns the RtpTransceiver tracks
// A RTPReceiver to support Simulcast may now have multiple tracks.
func (r *RTPReceiver) Tracks() []*TrackRemote {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tracks []*TrackRemote
	for i := range r.tracks {
		tracks = append(tracks, r.tracks[i].track)
	}

	return tracks
}

// RTPTransceiver returns the RTPTransceiver this
// RTPReceiver belongs too, or nil if none.
func (r *RTPReceiver) RTPTransceiver() *RTPTransceiver {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.tr
}

// configureReceive initialize the track.
func (r *RTPReceiver) configureReceive(parameters RTPReceiveParameters) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range parameters.Encodings {
		t := trackStreams{
			track: newTrackRemote(
				r.kind,
				parameters.Encodings[i].SSRC,
				parameters.Encodings[i].RTX.SSRC,
				parameters.Encodings[i].RID,
				r,
			),
		}

		r.tracks = append(r.tracks, t)
	}
}

// startReceive starts all the transports.
func (r *RTPReceiver) startReceive(parameters RTPReceiveParameters) error { //nolint:cyclop
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-r.received:
		return errRTPReceiverReceiveAlreadyCalled
	default:
	}

	globalParams := r.getParameters()
	codec := RTPCodecCapability{}
	if len(globalParams.Codecs) != 0 {
		codec = globalParams.Codecs[0].RTPCodecCapability
	}

	for i := range parameters.Encodings {
		if parameters.Encodings[i].RID != "" {
			// RID based tracks will be set up in receiveForRid
			continue
		}

		var streams *trackStreams
		for idx, ts := range r.tracks {
			if ts.track != nil && ts.track.SSRC() == parameters.Encodings[i].SSRC {
				streams = &r.tracks[idx]

				break
			}
		}
		if streams == nil {
			return fmt.Errorf("%w: %d", errRTPReceiverWithSSRCTrackStreamNotFound, parameters.Encodings[i].SSRC)
		}

		streams.streamInfo = createStreamInfo(
			"",
			parameters.Encodings[i].SSRC,
			0, 0, 0, 0, 0,
			codec,
			globalParams.HeaderExtensions,
		)

		result, err := r.transport.streamsForSSRC(parameters.Encodings[i].SSRC, *streams.streamInfo)
		if err != nil {
			return err
		}
		streams.rtpReadStream = result.rtpReadStream
		streams.rtpInterceptor = result.rtpInterceptor
		streams.rtpInterceptorWrapped = result.rtpInterceptorWrapped
		streams.rtcpReadStream = result.rtcpReadStream
		streams.rtcpInterceptor = result.rtcpInterceptor

		if rtxSsrc := parameters.Encodings[i].RTX.SSRC; rtxSsrc != 0 {
			// See RFC 4588 section 6.3,
			// NACKs MUST be sent only for the original RTP stream.
			rtxCodec := codec
			rtxCodec.RTCPFeedback = nil
			rtxCodec.MimeType = MimeTypeRTX
			streamInfo := createStreamInfo("", rtxSsrc, 0, 0, 0, 0, 0, rtxCodec, globalParams.HeaderExtensions)
			result, err = r.transport.streamsForSSRC(
				rtxSsrc,
				*streamInfo,
			)
			if err != nil {
				return err
			}
			rtpReadStream := result.rtpReadStream
			rtpInterceptor := result.rtpInterceptor
			rtpInterceptorWrapped := result.rtpInterceptorWrapped
			rtcpReadStream := result.rtcpReadStream
			rtcpInterceptor := result.rtcpInterceptor

			if err = r.receiveForRtxInternal(
				rtxSsrc,
				"",
				streamInfo,
				rtpReadStream,
				rtpInterceptor,
				rtpInterceptorWrapped,
				rtcpReadStream,
				rtcpInterceptor,
			); err != nil {
				return err
			}
		}
	}

	close(r.received)

	return nil
}

// Receive initialize the track and starts all the transports.
func (r *RTPReceiver) Receive(parameters RTPReceiveParameters) error {
	r.configureReceive(parameters)

	return r.startReceive(parameters)
}

// Read reads incoming RTCP for this RTPReceiver.
func (r *RTPReceiver) Read(b []byte) (n int, a interceptor.Attributes, err error) {
	select {
	case <-r.received:
		if len(r.tracks) > 1 {
			r.log.Errorf(useReadSimulcast)
		}

		return r.tracks[0].rtcpInterceptor.Read(b, a)
	case <-r.closedChan:
		return 0, nil, io.ErrClosedPipe
	}
}

// ReadSimulcast reads incoming RTCP for this RTPReceiver for given rid.
func (r *RTPReceiver) ReadSimulcast(b []byte, rid string) (n int, a interceptor.Attributes, err error) {
	select {
	case <-r.received:
		var rtcpInterceptor interceptor.RTCPReader

		r.mu.Lock()
		for _, t := range r.tracks {
			if t.track != nil && t.track.rid == rid {
				rtcpInterceptor = t.rtcpInterceptor
			}
		}
		r.mu.Unlock()

		if rtcpInterceptor == nil {
			return 0, nil, fmt.Errorf("%w: %s", errRTPReceiverForRIDTrackStreamNotFound, rid)
		}

		return rtcpInterceptor.Read(b, a)

	case <-r.closedChan:
		return 0, nil, io.ErrClosedPipe
	}
}

// ReadRTCP is a convenience method that wraps Read and unmarshal for you.
// It also runs any configured interceptors.
func (r *RTPReceiver) ReadRTCP() ([]rtcp.Packet, interceptor.Attributes, error) {
	b := make([]byte, r.api.settingEngine.getReceiveMTU())
	i, attributes, err := r.Read(b)
	if err != nil {
		return nil, nil, err
	}

	pkts, err := rtcp.Unmarshal(b[:i])
	if err != nil {
		return nil, nil, err
	}

	return pkts, attributes, nil
}

// ReadSimulcastRTCP is a convenience method that wraps ReadSimulcast and unmarshal for you.
func (r *RTPReceiver) ReadSimulcastRTCP(rid string) ([]rtcp.Packet, interceptor.Attributes, error) {
	b := make([]byte, r.api.settingEngine.getReceiveMTU())
	i, attributes, err := r.ReadSimulcast(b, rid)
	if err != nil {
		return nil, nil, err
	}

	pkts, err := rtcp.Unmarshal(b[:i])

	return pkts, attributes, err
}

func (r *RTPReceiver) haveReceived() bool {
	select {
	case <-r.received:
		return true
	default:
		return false
	}
}

func (r *RTPReceiver) haveClosed() bool {
	return r.closed.Load()
}

// Stop irreversibly stops the RTPReceiver.
func (r *RTPReceiver) Stop() error { //nolint:cyclop
	r.mu.Lock()
	defer r.mu.Unlock()
	var err error

	select {
	case <-r.closedChan:
		return err
	default:
	}

	select {
	case <-r.received:
		for i := range r.tracks {
			errs := []error{}

			if r.tracks[i].rtcpReadStream != nil {
				errs = append(errs, r.tracks[i].rtcpReadStream.Close())
			}

			if r.tracks[i].rtpReadStream != nil {
				errs = append(errs, r.tracks[i].rtpReadStream.Close())
			}

			if r.tracks[i].repairReadStream != nil {
				errs = append(errs, r.tracks[i].repairReadStream.Close())
			}

			if r.tracks[i].repairRtcpReadStream != nil {
				errs = append(errs, r.tracks[i].repairRtcpReadStream.Close())
			}

			if r.tracks[i].streamInfo != nil {
				r.api.interceptor.UnbindRemoteStream(r.tracks[i].streamInfo)
			}

			if r.tracks[i].repairStreamInfo != nil {
				r.api.interceptor.UnbindRemoteStream(r.tracks[i].repairStreamInfo)
			}

			err = util.FlattenErrs(errs)
		}
	default:
	}

	close(r.closedChan)
	r.closed.Store(true)

	return err
}

func (r *RTPReceiver) collectStats(collector *statsReportCollector, statsGetter stats.Getter) {
	if statsGetter == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Emit inbound-rtp stats for each track
	mid := ""
	if r.tr != nil {
		mid = r.tr.Mid()
	}
	now := statsTimestampNow()
	nowTime := now.Time()
	for trackIndex := range r.tracks {
		remoteTrack := r.tracks[trackIndex].track
		if remoteTrack == nil {
			continue
		}

		collector.Collecting()

		inboundID := fmt.Sprintf("inbound-rtp-%d", uint32(remoteTrack.SSRC()))
		codecID := ""
		if remoteTrack.codec.statsID != "" {
			codecID = remoteTrack.codec.statsID
		}

		inboundStats := InboundRTPStreamStats{
			Rid:         remoteTrack.RID(),
			Mid:         mid,
			Timestamp:   now,
			Type:        StatsTypeInboundRTP,
			ID:          inboundID,
			SSRC:        remoteTrack.SSRC(),
			Kind:        r.kind.String(),
			TransportID: "iceTransport",
			CodecID:     codecID,
		}
		r.populateInboundStats(&inboundStats, statsGetter, remoteTrack)

		collector.Collect(inboundID, inboundStats)

		if remoteTrack.Kind() == RTPCodecTypeAudio {
			r.collectAudioPlayoutStats(collector, nowTime, remoteTrack)
		}
	}
}

func (r *RTPReceiver) populateInboundStats(
	inboundStats *InboundRTPStreamStats,
	statsGetter stats.Getter,
	remoteTrack *TrackRemote,
) {
	stats := statsGetter.Get(uint32(remoteTrack.SSRC()))
	if stats == nil {
		return
	}

	// Wrap-around casting by design, with warnings if overflow/underflow is detected.
	pr := stats.InboundRTPStreamStats.PacketsReceived
	if pr > math.MaxUint32 {
		r.log.Warnf("Inbound PacketsReceived exceeds uint32 and will wrap: %d", pr)
	}
	inboundStats.PacketsReceived = uint32(pr) //nolint:gosec

	pl := stats.InboundRTPStreamStats.PacketsLost
	if pl > math.MaxInt32 || pl < math.MinInt32 {
		r.log.Warnf("Inbound PacketsLost exceeds int32 range and will wrap: %d", pl)
	}
	inboundStats.PacketsLost = int32(pl) //nolint:gosec

	inboundStats.Jitter = stats.InboundRTPStreamStats.Jitter
	inboundStats.BytesReceived = stats.InboundRTPStreamStats.BytesReceived
	inboundStats.HeaderBytesReceived = stats.InboundRTPStreamStats.HeaderBytesReceived
	timestamp := stats.InboundRTPStreamStats.LastPacketReceivedTimestamp
	inboundStats.LastPacketReceivedTimestamp = StatsTimestamp(
		timestamp.UnixNano() / int64(time.Millisecond))
	inboundStats.FIRCount = stats.InboundRTPStreamStats.FIRCount
	inboundStats.PLICount = stats.InboundRTPStreamStats.PLICount
	inboundStats.NACKCount = stats.InboundRTPStreamStats.NACKCount
}

func (r *RTPReceiver) collectAudioPlayoutStats(
	collector *statsReportCollector,
	nowTime time.Time,
	remoteTrack *TrackRemote,
) {
	playoutStats := remoteTrack.pullAudioPlayoutStats(nowTime)
	for _, stats := range playoutStats {
		collector.Collecting()
		collector.Collect(stats.ID, stats)
	}
}

func (r *RTPReceiver) streamsForTrack(t *TrackRemote) *trackStreams {
	for i := range r.tracks {
		if r.tracks[i].track == t {
			return &r.tracks[i]
		}
	}

	return nil
}

// readRTP should only be called by a track, this only exists so we can keep state in one place.
func (r *RTPReceiver) readRTP(b []byte, reader *TrackRemote) (n int, a interceptor.Attributes, err error) {
	select {
	case <-r.received:
	case <-r.closedChan:
		return 0, nil, io.EOF
	}

	r.mu.RLock()
	track := r.streamsForTrack(reader)
	var rtpInterceptor interceptor.RTPReader
	if track != nil {
		rtpInterceptor = track.rtpInterceptor
	}
	r.mu.RUnlock()
	if rtpInterceptor != nil {
		return rtpInterceptor.Read(b, a)
	}

	return 0, nil, fmt.Errorf("%w: %d", errRTPReceiverWithSSRCTrackStreamNotFound, reader.SSRC())
}

// receiveForRid is the sibling of Receive expect for RIDs instead of SSRCs
// It populates all the internal state for the given RID.
func (r *RTPReceiver) receiveForRid(
	rid string,
	params RTPParameters,
	streamInfo *interceptor.StreamInfo,
	rtpReadStream *srtp.ReadStreamSRTP,
	rtpInterceptor interceptor.RTPReader,
	rtpInterceptorWrapped bool,
	rtcpReadStream *srtp.ReadStreamSRTCP,
	rtcpInterceptor interceptor.RTCPReader,
	peekedPackets []*peekedPacket,
) (*TrackRemote, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.haveClosed() {
		return nil, io.EOF
	}

	for i := range r.tracks {
		if r.tracks[i].track.RID() == rid {
			r.tracks[i].track.mu.Lock()
			r.tracks[i].track.kind = r.kind
			r.tracks[i].track.codec = params.Codecs[0]
			r.tracks[i].track.params = params
			r.tracks[i].track.ssrc = SSRC(streamInfo.SSRC)
			r.tracks[i].track.peekedPackets = peekedPackets
			r.tracks[i].track.mu.Unlock()

			r.tracks[i].streamInfo = streamInfo
			r.tracks[i].rtpReadStream = rtpReadStream
			r.tracks[i].rtpInterceptor = rtpInterceptor
			r.tracks[i].rtpInterceptorWrapped = rtpInterceptorWrapped
			r.tracks[i].rtcpReadStream = rtcpReadStream
			r.tracks[i].rtcpInterceptor = rtcpInterceptor
			r.maybeStartRepairStreamReader(&r.tracks[i])

			return r.tracks[i].track, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", errRTPReceiverForRIDTrackStreamNotFound, rid)
}

// receiveForRtx configures the repair stream and starts a reader when needed.
func (r *RTPReceiver) receiveForRtx(
	ssrc SSRC,
	rsid string,
	streamInfo *interceptor.StreamInfo,
	rtpReadStream *srtp.ReadStreamSRTP,
	rtpInterceptor interceptor.RTPReader,
	rtpInterceptorWrapped bool,
	rtcpReadStream *srtp.ReadStreamSRTCP,
	rtcpInterceptor interceptor.RTCPReader,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.receiveForRtxInternal(
		ssrc,
		rsid,
		streamInfo,
		rtpReadStream,
		rtpInterceptor,
		rtpInterceptorWrapped,
		rtcpReadStream,
		rtcpInterceptor,
	)
}

//nolint:gocognit,cyclop
func (r *RTPReceiver) receiveForRtxInternal(
	ssrc SSRC,
	rsid string,
	streamInfo *interceptor.StreamInfo,
	rtpReadStream *srtp.ReadStreamSRTP,
	rtpInterceptor interceptor.RTPReader,
	rtpInterceptorWrapped bool,
	rtcpReadStream *srtp.ReadStreamSRTCP,
	rtcpInterceptor interceptor.RTCPReader,
) error {
	if r.haveClosed() {
		return io.EOF
	}

	var track *trackStreams
	if ssrc != 0 && len(r.tracks) == 1 {
		track = &r.tracks[0]
	} else {
		for i := range r.tracks {
			if r.tracks[i].track.RID() == rsid {
				track = &r.tracks[i]
				if track.track.RtxSSRC() == 0 {
					track.track.setRtxSSRC(SSRC(streamInfo.SSRC))
				}

				break
			}
		}
	}

	if track == nil {
		return fmt.Errorf("%w: ssrc(%d) rsid(%s)", errRTPReceiverForRIDTrackStreamNotFound, ssrc, rsid)
	}

	track.repairStreamInfo = streamInfo
	track.repairReadStream = rtpReadStream
	track.repairInterceptor = rtpInterceptor
	track.repairInterceptorWrapped = rtpInterceptorWrapped
	track.repairRtcpReadStream = rtcpReadStream
	track.repairRtcpInterceptor = rtcpInterceptor
	track.repairStreamGeneration++
	drainRepairStreamChannel(track.repairStreamChannel)
	r.maybeStartRepairStreamReader(track)

	return nil
}

func drainRepairStreamChannel(ch chan rtxPacketWithAttributes) {
	for {
		select {
		case packet, ok := <-ch:
			if !ok {
				return
			}
			packet.release()
		default:
			return
		}
	}
}

// maybeStartRepairStreamReader starts repair processing when an interceptor needs
// the stream, or after the application has requested packets from TrackRemote.
// The caller must hold r.mu.
func (r *RTPReceiver) maybeStartRepairStreamReader(track *trackStreams) {
	shouldStart := track.publicRepairReadRequested ||
		(!r.usesCustomBufferFactory() && (track.rtpInterceptorWrapped || track.repairInterceptorWrapped))
	if !shouldStart || track.repairInterceptor == nil ||
		track.repairReaderStartedGeneration == track.repairStreamGeneration {
		return
	}

	if track.repairStreamChannel == nil {
		track.repairStreamChannel = make(chan rtxPacketWithAttributes, 50)
	}

	remoteTrack := track.track
	repairInterceptor := track.repairInterceptor
	repairStreamChannel := track.repairStreamChannel
	generation := track.repairStreamGeneration
	track.repairReaderStartedGeneration = generation

	go r.runRepairStreamReader(remoteTrack, repairInterceptor, repairStreamChannel, generation)
}

func (r *RTPReceiver) runRepairStreamReader(
	remoteTrack *TrackRemote,
	repairInterceptor interceptor.RTPReader,
	repairStreamChannel chan rtxPacketWithAttributes,
	generation uint64,
) {
	for {
		if !r.isCurrentRepairStream(remoteTrack, generation) {
			return
		}

		b := r.rtxPool.Get().([]byte) // nolint:forcetypeassert
		i, attributes, err := repairInterceptor.Read(b, nil)
		if err != nil {
			r.rtxPool.Put(b) // nolint:staticcheck

			return
		}
		if !r.isCurrentRepairStream(remoteTrack, generation) {
			r.rtxPool.Put(b) // nolint:staticcheck

			return
		}

		packet, ok := r.rewriteRepairPacket(remoteTrack, b, i, attributes, generation)
		if !ok {
			// BWE probe packet, ignore
			r.rtxPool.Put(b) // nolint:staticcheck

			continue
		}
		select {
		case <-r.closedChan:
			packet.release()

			return
		case repairStreamChannel <- packet:
		default:
			// skip the RTX packet if the repair stream channel is full, could be blocked in the application's read loop
			packet.release()
		}
	}
}

// rewriteRepairPacket converts an RTX packet to the associated primary RTP stream.
func (r *RTPReceiver) rewriteRepairPacket(
	remoteTrack *TrackRemote,
	b []byte,
	packetLength int,
	attributes interceptor.Attributes,
	generation uint64,
) (rtxPacketWithAttributes, bool) {
	hasExtension := b[0]&0b10000 > 0
	hasPadding := b[0]&0b100000 > 0
	csrcCount := b[0] & 0b1111
	headerLength := uint16(12 + (4 * csrcCount))
	paddingLength := 0
	if hasExtension {
		headerLength += 4 * (1 + binary.BigEndian.Uint16(b[headerLength+2:headerLength+4]))
	}
	if hasPadding {
		paddingLength = int(b[packetLength-1])
	}
	if packetLength-int(headerLength)-paddingLength < 2 {
		return rtxPacketWithAttributes{}, false
	}

	if attributes == nil {
		attributes = make(interceptor.Attributes)
	}
	attributes.Set(AttributeRtxPayloadType, b[1]&0x7F)
	attributes.Set(AttributeRtxSequenceNumber, binary.BigEndian.Uint16(b[2:4]))
	attributes.Set(AttributeRtxSsrc, binary.BigEndian.Uint32(b[8:12]))

	b[1] = (b[1] & 0x80) | uint8(remoteTrack.PayloadType())
	b[2] = b[headerLength]
	b[3] = b[headerLength+1]
	binary.BigEndian.PutUint32(b[8:12], uint32(remoteTrack.SSRC()))
	copy(b[headerLength:packetLength-2], b[headerLength+2:packetLength])

	return rtxPacketWithAttributes{
		pkt:        b[:packetLength-2],
		attributes: attributes,
		pool:       &r.rtxPool,
		generation: generation,
	}, true
}

func (r *RTPReceiver) isCurrentRepairStream(reader *TrackRemote, generation uint64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	track := r.streamsForTrack(reader)

	return track != nil && track.repairStreamGeneration == generation
}

// SetReadDeadline sets the max amount of time the RTCP stream will block before returning. 0 is forever.
func (r *RTPReceiver) SetReadDeadline(t time.Time) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.tracks[0].rtcpReadStream.SetReadDeadline(t)
}

// SetReadDeadlineSimulcast sets the max amount of time the RTCP stream for a given rid will block before returning.
// 0 is forever.
func (r *RTPReceiver) SetReadDeadlineSimulcast(deadline time.Time, rid string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, t := range r.tracks {
		if t.track != nil && t.track.rid == rid {
			return t.rtcpReadStream.SetReadDeadline(deadline)
		}
	}

	return fmt.Errorf("%w: %s", errRTPReceiverForRIDTrackStreamNotFound, rid)
}

// setRTPReadDeadline sets the max amount of time the RTP stream will block before returning. 0 is forever.
// This should be fired by calling SetReadDeadline on the TrackRemote.
func (r *RTPReceiver) setRTPReadDeadline(deadline time.Time, reader *TrackRemote) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if t := r.streamsForTrack(reader); t != nil {
		return t.rtpReadStream.SetReadDeadline(deadline)
	}

	return fmt.Errorf("%w: %d", errRTPReceiverWithSSRCTrackStreamNotFound, reader.SSRC())
}

// requestRepairStreamReader records that TrackRemote packets are being consumed.
// The returned channel remains stable across repair stream binds and rebinds.
func (r *RTPReceiver) requestRepairStreamReader(reader *TrackRemote) chan rtxPacketWithAttributes {
	if r.haveClosed() {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.haveClosed() {
		return nil
	}

	track := r.streamsForTrack(reader)
	if track == nil {
		return nil
	}

	track.publicRepairReadRequested = true
	if track.repairStreamChannel == nil {
		track.repairStreamChannel = make(chan rtxPacketWithAttributes, 50)
	}
	r.maybeStartRepairStreamReader(track)

	return track.repairStreamChannel
}

// readRTX returns an RTX packet if one is available on the RTX track, otherwise returns nil.
func (r *RTPReceiver) readRTX(reader *TrackRemote) *rtxPacketWithAttributes {
	if !reader.HasRTX() || r.haveClosed() || !r.haveReceived() {
		return nil
	}

	r.mu.RLock()
	track := r.streamsForTrack(reader)
	var ch chan rtxPacketWithAttributes
	if track != nil {
		ch = track.repairStreamChannel
	}
	r.mu.RUnlock()

	return r.readRTXFromChannel(reader, ch)
}

func (r *RTPReceiver) readRTXFromChannel(
	reader *TrackRemote,
	ch chan rtxPacketWithAttributes,
) *rtxPacketWithAttributes {
	for {
		select {
		case packet, ok := <-ch:
			if !ok {
				return nil
			}
			if r.isCurrentRepairStream(reader, packet.generation) {
				return &packet
			}
			packet.release()
		default:
			return nil
		}
	}
}
