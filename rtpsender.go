// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/randutil"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/internal/util"
)

type trackEncoding struct {
	track TrackLocal

	srtpStream *srtpWriterFuture

	rtcpInterceptor interceptor.RTCPReader
	streamInfo      interceptor.StreamInfo

	context *baseTrackLocalContext

	ssrc SSRC
}

// RTPSender allows an application to control how a given Track is encoded and transmitted to a remote peer
type RTPSender struct {
	trackEncodings []*trackEncoding

	transport *DTLSTransport

	payloadType PayloadType
	kind        RTPCodecType

	// nolint:godox
	// TODO(sgotti) remove this when in future we'll avoid replacing
	// a transceiver sender since we can just check the
	// transceiver negotiation status
	negotiated bool

	// A reference to the associated api object
	api *API
	id  string

	rtpTransceiver *RTPTransceiver

	mu                     sync.RWMutex
	sendCalled, stopCalled chan struct{}
}

// NewRTPSender constructs a new RTPSender
func (api *API) NewRTPSender(track TrackLocal, transport *DTLSTransport) (*RTPSender, error) {
	if track == nil {
		return nil, errRTPSenderTrackNil
	} else if transport == nil {
		return nil, errRTPSenderDTLSTransportNil
	}

	id, err := randutil.GenerateCryptoRandomString(32, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if err != nil {
		return nil, err
	}

	r := &RTPSender{
		transport:  transport,
		api:        api,
		sendCalled: make(chan struct{}),
		stopCalled: make(chan struct{}),
		id:         id,
		kind:       track.Kind(),
	}

	r.addEncoding(track)

	return r, nil
}

func (r *RTPSender) isNegotiated() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.negotiated
}

func (r *RTPSender) setNegotiated() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.negotiated = true
}

func (r *RTPSender) setRTPTransceiver(rtpTransceiver *RTPTransceiver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rtpTransceiver = rtpTransceiver
}

// Transport returns the currently-configured *DTLSTransport or nil
// if one has not yet been configured
func (r *RTPSender) Transport() *DTLSTransport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.transport
}

func (r *RTPSender) getParameters() RTPSendParameters {
	var encodings []RTPEncodingParameters
	for _, trackEncoding := range r.trackEncodings {
		var rid string
		if trackEncoding.track != nil {
			rid = trackEncoding.track.RID()
		}
		encodings = append(encodings, RTPEncodingParameters{
			RTPCodingParameters: RTPCodingParameters{
				RID:         rid,
				SSRC:        trackEncoding.ssrc,
				PayloadType: r.payloadType,
			},
		})
	}
	sendParameters := RTPSendParameters{
		RTPParameters: r.api.mediaEngine.getRTPParametersByKind(
			r.kind,
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendonly},
		),
		Encodings: encodings,
	}
	if r.rtpTransceiver != nil {
		sendParameters.Codecs = r.rtpTransceiver.getCodecs()
	} else {
		sendParameters.Codecs = r.api.mediaEngine.getCodecsByKind(r.kind)
	}
	return sendParameters
}

// GetParameters describes the current configuration for the encoding and
// transmission of media on the sender's track.
func (r *RTPSender) GetParameters() RTPSendParameters {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getParameters()
}

// AddEncoding adds an encoding to RTPSender. Used by simulcast senders.
func (r *RTPSender) AddEncoding(track TrackLocal) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if track == nil {
		return errRTPSenderTrackNil
	}

	if track.RID() == "" {
		return errRTPSenderRidNil
	}

	if r.hasStopped() {
		return errRTPSenderStopped
	}

	if r.hasSent() {
		return errRTPSenderSendAlreadyCalled
	}

	var refTrack TrackLocal
	if len(r.trackEncodings) != 0 {
		refTrack = r.trackEncodings[0].track
	}
	if refTrack == nil || refTrack.RID() == "" {
		return errRTPSenderNoBaseEncoding
	}

	if refTrack.ID() != track.ID() || refTrack.StreamID() != track.StreamID() || refTrack.Kind() != track.Kind() {
		return errRTPSenderBaseEncodingMismatch
	}

	for _, encoding := range r.trackEncodings {
		if encoding.track == nil {
			continue
		}

		if encoding.track.RID() == track.RID() {
			return errRTPSenderRIDCollision
		}
	}

	r.addEncoding(track)
	return nil
}

func (r *RTPSender) addEncoding(track TrackLocal) {
	ssrc := SSRC(randutil.NewMathRandomGenerator().Uint32())
	trackEncoding := &trackEncoding{
		track:      track,
		srtpStream: &srtpWriterFuture{ssrc: ssrc},
		ssrc:       ssrc,
	}
	trackEncoding.srtpStream.rtpSender = r
	trackEncoding.rtcpInterceptor = r.api.interceptor.BindRTCPReader(
		interceptor.RTPReaderFunc(func(in []byte, a interceptor.Attributes) (n int, attributes interceptor.Attributes, err error) {
			n, err = trackEncoding.srtpStream.Read(in)
			return n, a, err
		}),
	)

	r.trackEncodings = append(r.trackEncodings, trackEncoding)
}

// Track returns the RTCRtpTransceiver track, or nil
func (r *RTPSender) Track() TrackLocal {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.trackEncodings) == 0 {
		return nil
	}

	return r.trackEncodings[0].track
}

// ReplaceTrack replaces the track currently being used as the sender's source with a new TrackLocal.
// The new track must be of the same media kind (audio, video, etc) and switching the track should not
// require negotiation.
func (r *RTPSender) ReplaceTrack(track TrackLocal) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if track != nil && r.kind != track.Kind() {
		return ErrRTPSenderNewTrackHasIncorrectKind
	}

	// cannot replace simulcast envelope
	if track != nil && len(r.trackEncodings) > 1 {
		return ErrRTPSenderNewTrackHasIncorrectEnvelope
	}

	var replacedTrack TrackLocal
	var context *baseTrackLocalContext
	if len(r.trackEncodings) != 0 {
		replacedTrack = r.trackEncodings[0].track
		context = r.trackEncodings[0].context
	}
	if r.hasSent() && replacedTrack != nil {
		if err := replacedTrack.Unbind(context); err != nil {
			return err
		}
	}

	if !r.hasSent() || track == nil {
		r.trackEncodings[0].track = track
		return nil
	}

	codec, err := track.Bind(&baseTrackLocalContext{
		id:              context.ID(),
		params:          r.api.mediaEngine.getRTPParametersByKind(track.Kind(), []RTPTransceiverDirection{RTPTransceiverDirectionSendonly}),
		ssrc:            context.SSRC(),
		writeStream:     context.WriteStream(),
		rtcpInterceptor: context.RTCPReader(),
	})
	if err != nil {
		// Re-bind the original track
		if _, reBindErr := replacedTrack.Bind(context); reBindErr != nil {
			return reBindErr
		}

		return err
	}

	// Codec has changed
	if r.payloadType != codec.PayloadType {
		context.params.Codecs = []RTPCodecParameters{codec}
	}

	r.trackEncodings[0].track = track
	return nil
}

// Send Attempts to set the parameters controlling the sending of media.
func (r *RTPSender) Send(parameters RTPSendParameters) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch {
	case r.hasSent():
		return errRTPSenderSendAlreadyCalled
	case r.trackEncodings[0].track == nil:
		return errRTPSenderTrackRemoved
	}

	for idx, trackEncoding := range r.trackEncodings {
		writeStream := &interceptorToTrackLocalWriter{}
		trackEncoding.context = &baseTrackLocalContext{
			id:              r.id,
			params:          r.api.mediaEngine.getRTPParametersByKind(trackEncoding.track.Kind(), []RTPTransceiverDirection{RTPTransceiverDirectionSendonly}),
			ssrc:            parameters.Encodings[idx].SSRC,
			writeStream:     writeStream,
			rtcpInterceptor: trackEncoding.rtcpInterceptor,
		}

		codec, err := trackEncoding.track.Bind(trackEncoding.context)
		if err != nil {
			return err
		}
		trackEncoding.context.params.Codecs = []RTPCodecParameters{codec}

		trackEncoding.streamInfo = *createStreamInfo(
			r.id,
			parameters.Encodings[idx].SSRC,
			codec.PayloadType,
			codec.RTPCodecCapability,
			parameters.HeaderExtensions,
		)
		srtpStream := trackEncoding.srtpStream
		rtpInterceptor := r.api.interceptor.BindLocalStream(
			&trackEncoding.streamInfo,
			interceptor.RTPWriterFunc(func(header *rtp.Header, payload []byte, attributes interceptor.Attributes) (int, error) {
				return srtpStream.WriteRTP(header, payload)
			}),
		)
		writeStream.interceptor.Store(rtpInterceptor)
	}

	close(r.sendCalled)
	return nil
}

// Stop irreversibly stops the RTPSender
func (r *RTPSender) Stop() error {
	r.mu.Lock()

	if stopped := r.hasStopped(); stopped {
		r.mu.Unlock()
		return nil
	}

	close(r.stopCalled)
	r.mu.Unlock()

	if !r.hasSent() {
		return nil
	}

	if err := r.ReplaceTrack(nil); err != nil {
		return err
	}

	errs := []error{}
	for _, trackEncoding := range r.trackEncodings {
		r.api.interceptor.UnbindLocalStream(&trackEncoding.streamInfo)
		errs = append(errs, trackEncoding.srtpStream.Close())
	}

	return util.FlattenErrs(errs)
}

// Read reads incoming RTCP for this RTPSender
func (r *RTPSender) Read(b []byte) (n int, a interceptor.Attributes, err error) {
	select {
	case <-r.sendCalled:
		return r.trackEncodings[0].rtcpInterceptor.Read(b, a)
	case <-r.stopCalled:
		return 0, nil, io.ErrClosedPipe
	}
}

// ReadRTCP is a convenience method that wraps Read and unmarshals for you.
func (r *RTPSender) ReadRTCP() ([]rtcp.Packet, interceptor.Attributes, error) {
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

// ReadSimulcast reads incoming RTCP for this RTPSender for given rid
func (r *RTPSender) ReadSimulcast(b []byte, rid string) (n int, a interceptor.Attributes, err error) {
	select {
	case <-r.sendCalled:
		for _, t := range r.trackEncodings {
			if t.track != nil && t.track.RID() == rid {
				return t.rtcpInterceptor.Read(b, a)
			}
		}
		return 0, nil, fmt.Errorf("%w: %s", errRTPSenderNoTrackForRID, rid)
	case <-r.stopCalled:
		return 0, nil, io.ErrClosedPipe
	}
}

// ReadSimulcastRTCP is a convenience method that wraps ReadSimulcast and unmarshal for you
func (r *RTPSender) ReadSimulcastRTCP(rid string) ([]rtcp.Packet, interceptor.Attributes, error) {
	b := make([]byte, r.api.settingEngine.getReceiveMTU())
	i, attributes, err := r.ReadSimulcast(b, rid)
	if err != nil {
		return nil, nil, err
	}

	pkts, err := rtcp.Unmarshal(b[:i])
	return pkts, attributes, err
}

// SetReadDeadline sets the deadline for the Read operation.
// Setting to zero means no deadline.
func (r *RTPSender) SetReadDeadline(t time.Time) error {
	return r.trackEncodings[0].srtpStream.SetReadDeadline(t)
}

// SetReadDeadlineSimulcast sets the max amount of time the RTCP stream for a given rid will block before returning. 0 is forever.
func (r *RTPSender) SetReadDeadlineSimulcast(deadline time.Time, rid string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, t := range r.trackEncodings {
		if t.track != nil && t.track.RID() == rid {
			return t.srtpStream.SetReadDeadline(deadline)
		}
	}
	return fmt.Errorf("%w: %s", errRTPSenderNoTrackForRID, rid)
}

// hasSent tells if data has been ever sent for this instance
func (r *RTPSender) hasSent() bool {
	select {
	case <-r.sendCalled:
		return true
	default:
		return false
	}
}

// hasStopped tells if stop has been called
func (r *RTPSender) hasStopped() bool {
	select {
	case <-r.stopCalled:
		return true
	default:
		return false
	}
}
