// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/srtp/v3"
	"github.com/pion/webrtc/v5/internal/util"
)

// trackStreams maintains a mapping of RTP/RTCP streams to a specific track
// a RTPReceiver may contain multiple streams if we are dealing with Simulcast.
type trackStreams struct {
	track *TrackRemote

	streamInfo, repairStreamInfo *interceptor.StreamInfo

	rtpInterceptor interceptor.RTPReader

	rtcpReadStream  *srtp.ReadStreamSRTCP
	rtcpInterceptor interceptor.RTCPReader

	rtpReader            *srtpRTPReader
	repairReader         *srtpRTPReader
	repairRtcpReadStream *srtp.ReadStreamSRTCP
}

// RTPReceiver allows an application to inspect the receipt of a TrackRemote.
type RTPReceiver struct {
	kind      RTPCodecType
	transport *DTLSTransport

	tracks []trackStreams

	closed     atomic.Bool
	closedChan chan struct{}
	received   chan any
	mu         sync.RWMutex

	tr *RTPTransceiver

	// A reference to the associated api object
	api *API

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
		closedChan: make(chan struct{}),
		received:   make(chan any),
		tracks:     []trackStreams{},
		log:        api.settingEngine.LoggerFactory.NewLogger("RTPReceiver"),
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
		streams.rtpInterceptor = result.rtpInterceptor
		streams.rtcpReadStream = result.rtcpReadStream
		streams.rtcpInterceptor = result.rtcpInterceptor
		streams.rtpReader, _ = result.rtpInterceptor.(*srtpRTPReader)

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

			if err = r.receiveForRtxInternal(rtxSsrc, "", streamInfo, result); err != nil {
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

		return r.ReadSimulcast(b, r.tracks[0].track.RID())
	case <-r.closedChan:
		return 0, nil, io.ErrClosedPipe
	}
}

// ReadSimulcast reads incoming RTCP for this RTPReceiver for given rid.
func (r *RTPReceiver) ReadSimulcast(b []byte, rid string) (n int, a interceptor.Attributes, err error) {
	select {
	case <-r.received:
		for _, track := range r.Tracks() {
			if track.RID() == rid {
				if err := track.streamFuture.wait(track.rtcpReadDeadline); err != nil {
					return 0, nil, err
				}
				r.mu.RLock()
				reader := r.streamsForTrack(track).rtcpInterceptor
				r.mu.RUnlock()

				return reader.Read(b, a)
			}
		}

		return 0, nil, fmt.Errorf("%w: %s", errRTPReceiverForRIDTrackStreamNotFound, rid)

	case <-r.closedChan:
		return 0, nil, io.ErrClosedPipe
	}
}

// ReadSimulcastSSRC reads incoming RTCP for this RTPReceiver for given SSRC.
func (r *RTPReceiver) ReadSimulcastSSRC(b []byte, ssrc SSRC) (n int, a interceptor.Attributes, err error) {
	select {
	case <-r.received:
		var rtcpInterceptor interceptor.RTCPReader

		r.mu.Lock()
		for _, t := range r.tracks {
			if t.track != nil && t.track.ssrc == ssrc {
				rtcpInterceptor = t.rtcpInterceptor

				break
			}
		}
		r.mu.Unlock()

		if rtcpInterceptor == nil {
			return 0, nil, fmt.Errorf("%w: %d", errRTPReceiverForSSRCTrackStreamNotFound, ssrc)
		}

		return rtcpInterceptor.Read(b, a)

	case <-r.closedChan:
		return 0, nil, io.ErrClosedPipe
	}
}

// ReadRTCP is a convenience method that wraps Read and unmarshal for you.
// It also runs any configured interceptors.
func (r *RTPReceiver) ReadRTCP() ([]rtcp.Packet, interceptor.Attributes, error) {
	return readRTCP(r.Read, r.api.settingEngine.getReceiveMTU())
}

func readRTCP(read func([]byte) (int, interceptor.Attributes, error), mtu uint) ([]rtcp.Packet, interceptor.Attributes, error) {
	b := make([]byte, mtu)
	i, attributes, err := read(b)
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

// ReadSimulcastSSRCRTCP is a convenience method that wraps ReadSimulcastSSRC and unmarshal for you.
func (r *RTPReceiver) ReadSimulcastSSRCRTCP(ssrc SSRC) ([]rtcp.Packet, interceptor.Attributes, error) {
	return readRTCP(func(b []byte) (int, interceptor.Attributes, error) {
		return r.ReadSimulcastSSRC(b, ssrc)
	}, r.api.settingEngine.getReceiveMTU())
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

			if r.tracks[i].rtpReader != nil {
				errs = append(errs, r.tracks[i].rtpReader.readStream.Close())
			}

			if r.tracks[i].repairReader != nil {
				errs = append(errs, r.tracks[i].repairReader.readStream.Close())
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
	for _, streams := range r.tracks {
		if track := streams.track; track.streamsReadyCancel != nil {
			track.rtpReadDeadline.Set(time.Time{})
			track.rtcpReadDeadline.Set(time.Time{})
			track.streamsReadyCancel()
		}
	}

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
			Rid:             remoteTrack.RID(),
			Mid:             mid,
			Timestamp:       now,
			Type:            StatsTypeInboundRTP,
			ID:              inboundID,
			SSRC:            remoteTrack.SSRC(),
			Kind:            r.kind.String(),
			TrackIdentifier: remoteTrack.ID(),
			TransportID:     "iceTransport",
			CodecID:         codecID,
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

	inboundStats.PacketsReceived = stats.InboundRTPStreamStats.PacketsReceived
	inboundStats.PacketsLost = stats.InboundRTPStreamStats.PacketsLost
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
func (r *RTPReceiver) readRTP(b []byte, reader *TrackRemote) (n int, a interceptor.Attributes, err error) { //nolint:cyclop
	select {
	case <-r.received:
	case <-r.closedChan:
		return 0, nil, io.EOF
	}

	r.mu.RLock()
	var rtpInterceptor interceptor.RTPReader
	if t := r.streamsForTrack(reader); t != nil {
		rtpInterceptor = t.rtpInterceptor
	}
	r.mu.RUnlock()
	if rtpInterceptor == nil {
		return 0, nil, fmt.Errorf("%w: %d", errRTPReceiverWithSSRCTrackStreamNotFound, reader.SSRC())
	}
	for {
		n, a, err = rtpInterceptor.Read(b, nil)
		if err != nil {
			return n, a, err
		}
		if n < 12 {
			continue
		}
		if reader.RtxSSRC() == 0 || binary.BigEndian.Uint32(b[8:12]) != uint32(reader.RtxSSRC()) {
			return n, a, nil
		}
		var packet rtp.Packet
		if packet.Unmarshal(b[:n]) != nil || len(packet.Payload) < 2 {
			continue
		}
		headerLength := n - len(packet.Payload) - int(packet.Header.PaddingSize)
		// Use the negotiated association, including when RTX arrives before primary RTP.
		payloadType := reader.PayloadType()
		codec, _, codecErr := r.api.mediaEngine.getCodecByPayload(PayloadType(packet.PayloadType))
		if codecErr == nil {
			if codec.rtxPayloadType == nil {
				continue
			}
			payloadType = *codec.rtxPayloadType
		}
		if a == nil {
			a = make(interceptor.Attributes)
		}
		a.Set(AttributeRtxPayloadType, packet.PayloadType)
		a.Set(AttributeRtxSequenceNumber, packet.SequenceNumber)
		a.Set(AttributeRtxSsrc, packet.SSRC)
		b[1] = (b[1] & 0x80) | uint8(payloadType)
		copy(b[2:4], b[headerLength:headerLength+2])
		binary.BigEndian.PutUint32(b[8:12], uint32(reader.SSRC()))
		copy(b[headerLength:n-2], b[headerLength+2:n])

		return n - 2, a, nil
	}
}

// receiveForRid is the sibling of Receive expect for RIDs instead of SSRCs
// It populates all the internal state for the given RID.
func (r *RTPReceiver) receiveForRid(
	rid string,
	params RTPParameters,
	streamInfo *interceptor.StreamInfo,
	streams *streamsForSSRCResult,
	peekedPackets []*peekedPacket,
) (*TrackRemote, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.haveClosed() {
		return nil, io.EOF
	}

	for i := range r.tracks {
		if r.tracks[i].track.RID() != rid {
			continue
		}
		r.tracks[i].track.mu.Lock()
		r.tracks[i].track.kind = r.kind
		r.tracks[i].track.codec = params.Codecs[0]
		r.tracks[i].track.params = params
		r.tracks[i].track.ssrc = SSRC(streamInfo.SSRC)
		r.tracks[i].track.peekedPackets = peekedPackets
		r.tracks[i].track.mu.Unlock()

		r.tracks[i].streamInfo = streamInfo
		r.tracks[i].rtpInterceptor = streams.rtpInterceptor
		r.tracks[i].rtcpReadStream = streams.rtcpReadStream
		r.tracks[i].rtcpInterceptor = streams.rtcpInterceptor
		r.tracks[i].rtpReader, _ = streams.rtpInterceptor.(*srtpRTPReader)
		if err := r.tracks[i].setRTX(); err != nil {
			return nil, err
		}
		track := r.tracks[i].track
		if r.tracks[i].rtpReader != nil {
			readDeadline, _ := track.rtpReadDeadline.Deadline()
			if err := r.tracks[i].rtpReader.readStream.SetReadDeadline(readDeadline); err != nil {
				return nil, err
			}
		}
		if streams.rtcpReadStream != nil {
			readDeadline, _ := track.rtcpReadDeadline.Deadline()
			if err := streams.rtcpReadStream.SetReadDeadline(readDeadline); err != nil {
				return nil, err
			}
		}
		track.streamsReadyCancel()

		return track, nil
	}

	return nil, fmt.Errorf("%w: %s", errRTPReceiverForRIDTrackStreamNotFound, rid)
}

// receiveForRtx associates the repair stream with its primary stream.
func (r *RTPReceiver) receiveForRtx(
	ssrc SSRC,
	rsid string,
	streamInfo *interceptor.StreamInfo,
	streams *streamsForSSRCResult,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.receiveForRtxInternal(ssrc, rsid, streamInfo, streams)
}

//nolint:cyclop,nestif
func (r *RTPReceiver) receiveForRtxInternal(
	ssrc SSRC,
	rsid string,
	streamInfo *interceptor.StreamInfo,
	streams *streamsForSSRCResult,
) error {
	if r.haveClosed() {
		return io.EOF
	}

	var track *trackStreams
	if ssrc != 0 && len(r.tracks) == 1 {
		track = &r.tracks[0]
	} else {
		for i := range r.tracks {
			if rsid != "" && r.tracks[i].track.RID() == rsid {
				track = &r.tracks[i]
				if track.track.RtxSSRC() == 0 {
					track.track.setRtxSSRC(SSRC(streamInfo.SSRC))
				}

				break
			} else if rsid == "" && r.tracks[i].track.RtxSSRC() == ssrc {
				track = &r.tracks[i]

				break
			}
		}
	}

	if track == nil {
		if rsid == "" {
			return fmt.Errorf("%w: %d", errRTPReceiverForSSRCTrackStreamNotFound, ssrc)
		}

		return fmt.Errorf("%w: ssrc(%d) rsid(%s)", errRTPReceiverForRIDTrackStreamNotFound, ssrc, rsid)
	}

	track.repairStreamInfo = streamInfo
	track.repairReader, _ = streams.rtpInterceptor.(*srtpRTPReader)
	track.repairRtcpReadStream = streams.rtcpReadStream

	return track.setRTX()
}

// setRTX is called with the receiver locked, after either stream is bound.
func (t *trackStreams) setRTX() error {
	if t.rtpReader == nil || t.repairReader == nil {
		return nil
	}
	t.rtpReader.repair.Store(t.repairReader)

	return t.rtpReader.readStream.SetRTX(t.repairReader.readStream.GetSSRC())
}

// SetReadDeadline sets the max amount of time the RTCP stream will block before returning. 0 is forever.
func (r *RTPReceiver) SetReadDeadline(t time.Time) error {
	r.mu.RLock()
	rid := r.tracks[0].track.RID()
	r.mu.RUnlock()

	return r.SetReadDeadlineSimulcast(t, rid)
}

// SetReadDeadlineSimulcast sets the max amount of time the RTCP stream for a given rid will block before returning.
// 0 is forever.
func (r *RTPReceiver) SetReadDeadlineSimulcast(deadline time.Time, rid string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, t := range r.tracks {
		if t.track != nil && t.track.rid == rid {
			if t.track.rtcpReadDeadline != nil {
				t.track.rtcpReadDeadline.Set(deadline)
				if t.rtcpReadStream == nil {
					return nil
				}
			}

			return t.rtcpReadStream.SetReadDeadline(deadline)
		}
	}

	return fmt.Errorf("%w: %s", errRTPReceiverForRIDTrackStreamNotFound, rid)
}

// SetReadDeadlineSimulcastSSRC sets the max amount of time the RTCP stream for a given SSRC will block before
// returning. 0 is forever.
func (r *RTPReceiver) SetReadDeadlineSimulcastSSRC(deadline time.Time, ssrc SSRC) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, t := range r.tracks {
		if t.track != nil && t.track.ssrc == ssrc && t.rtcpReadStream != nil {
			return t.rtcpReadStream.SetReadDeadline(deadline)
		}
	}

	return fmt.Errorf("%w: %d", errRTPReceiverForSSRCTrackStreamNotFound, ssrc)
}

// setRTPReadDeadline sets the max amount of time the RTP stream will block before returning. 0 is forever.
// This should be fired by calling SetReadDeadline on the TrackRemote.
func (r *RTPReceiver) setRTPReadDeadline(deadline time.Time, reader *TrackRemote) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if t := r.streamsForTrack(reader); t != nil {
		if reader.rtpReadDeadline != nil {
			reader.rtpReadDeadline.Set(deadline)
			if t.rtpReader == nil {
				return nil
			}
		}

		return t.rtpReader.readStream.SetReadDeadline(deadline)
	}

	return fmt.Errorf("%w: %d", errRTPReceiverWithSSRCTrackStreamNotFound, reader.SSRC())
}
