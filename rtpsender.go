// +build !js

package webrtc

import (
	"context"
	"io"
	"sync"

	"github.com/pion/interceptor"
	"github.com/pion/randutil"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

// RTPSender allows an application to control how a given Track is encoded and transmitted to a remote peer
type RTPSender struct {
	track TrackLocal

	srtpStream *srtpWriterFuture
	context    TrackLocalContext

	transport *DTLSTransport

	payloadType PayloadType
	ssrc        SSRC

	// nolint:godox
	// TODO(sgotti) remove this when in future we'll avoid replacing
	// a transceiver sender since we can just check the
	// transceiver negotiation status
	negotiated bool

	// A reference to the associated api object
	api *API
	id  string

	mu                     sync.RWMutex
	sendCalled, stopCalled chan struct{}

	interceptorRTCPReader interceptor.RTCPReader
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
		track:      track,
		transport:  transport,
		api:        api,
		sendCalled: make(chan struct{}),
		stopCalled: make(chan struct{}),
		ssrc:       SSRC(randutil.NewMathRandomGenerator().Uint32()),
		id:         id,
		srtpStream: &srtpWriterFuture{},
	}

	r.interceptorRTCPReader = api.interceptor.BindRTCPReader(interceptor.RTCPReaderFunc(r.readRTCP))
	r.srtpStream.rtpSender = r

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

// Transport returns the currently-configured *DTLSTransport or nil
// if one has not yet been configured
func (r *RTPSender) Transport() *DTLSTransport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.transport
}

// Track returns the RTCRtpTransceiver track, or nil
func (r *RTPSender) Track() TrackLocal {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.track
}

// ReplaceTrack replaces the track currently being used as the sender's source with a new TrackLocal.
// The new track must be of the same media kind (audio, video, etc) and switching the track should not
// require negotiation.
func (r *RTPSender) ReplaceTrack(track TrackLocal) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.hasSent() && r.track != nil {
		if err := r.track.Unbind(r.context); err != nil {
			return err
		}
	}

	if !r.hasSent() || track == nil {
		r.track = track
		return nil
	}

	if _, err := track.Bind(r.context); err != nil {
		// Re-bind the original track
		if _, reBindErr := r.track.Bind(r.context); reBindErr != nil {
			return reBindErr
		}

		return err
	}

	r.track = track
	return nil
}

// Send Attempts to set the parameters controlling the sending of media.
func (r *RTPSender) Send(parameters RTPSendParameters) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.hasSent() {
		return errRTPSenderSendAlreadyCalled
	}

	writeStream := &interceptorTrackLocalWriter{TrackLocalWriter: r.srtpStream}

	r.context = TrackLocalContext{
		id:          r.id,
		params:      r.api.mediaEngine.getRTPParametersByKind(r.track.Kind()),
		ssrc:        parameters.Encodings.SSRC,
		writeStream: writeStream,
	}

	codec, err := r.track.Bind(r.context)
	if err != nil {
		return err
	}
	r.context.params.Codecs = []RTPCodecParameters{codec}

	headerExtensions := make([]interceptor.RTPHeaderExtension, 0, len(r.context.params.HeaderExtensions))
	for _, h := range r.context.params.HeaderExtensions {
		headerExtensions = append(headerExtensions, interceptor.RTPHeaderExtension{ID: h.ID, URI: h.URI})
	}
	feedbacks := make([]interceptor.RTCPFeedback, 0, len(codec.RTCPFeedback))
	for _, f := range codec.RTCPFeedback {
		feedbacks = append(feedbacks, interceptor.RTCPFeedback{Type: f.Type, Parameter: f.Parameter})
	}
	info := &interceptor.StreamInfo{
		ID:                  r.context.id,
		Attributes:          interceptor.Attributes{},
		SSRC:                uint32(r.context.ssrc),
		PayloadType:         uint8(codec.PayloadType),
		RTPHeaderExtensions: headerExtensions,
		MimeType:            codec.MimeType,
		ClockRate:           codec.ClockRate,
		Channels:            codec.Channels,
		SDPFmtpLine:         codec.SDPFmtpLine,
		RTCPFeedback:        feedbacks,
	}
	writeStream.setRTPWriter(
		r.api.interceptor.BindLocalStream(
			info,
			interceptor.RTPWriterFunc(func(ctx context.Context, p *rtp.Packet, attributes interceptor.Attributes) (int, error) {
				return r.srtpStream.WriteRTP(ctx, &p.Header, p.Payload)
			}),
		))

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

	return r.srtpStream.Close()
}

// Read reads incoming RTCP for this RTPReceiver
func (r *RTPSender) Read(ctx context.Context, b []byte) (n int, err error) {
	select {
	case <-r.sendCalled:
		return r.srtpStream.ReadContext(ctx, b)
	case <-r.stopCalled:
		return 0, io.ErrClosedPipe
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// ReadRTCP is a convenience method that wraps Read and unmarshals for you.
// It also runs any configured interceptors.
func (r *RTPSender) ReadRTCP(ctx context.Context) ([]rtcp.Packet, error) {
	pkts, _, err := r.interceptorRTCPReader.Read(ctx)
	return pkts, err
}

func (r *RTPSender) readRTCP(ctx context.Context) ([]rtcp.Packet, interceptor.Attributes, error) {
	b := make([]byte, receiveMTU)
	i, err := r.Read(ctx, b)
	if err != nil {
		return nil, nil, err
	}

	pkts, err := rtcp.Unmarshal(b[:i])
	if err != nil {
		return nil, nil, err
	}

	return pkts, make(interceptor.Attributes), nil
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
