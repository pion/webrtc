// +build !js

package webrtc

import (
	"io"
	"sync"

	"github.com/pion/interceptor"
	"github.com/pion/randutil"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/srtp"
)

// RTPSender allows an application to control how a given Track is encoded and transmitted to a remote peer
type RTPSender struct {
	track TrackLocal

	rtcpReadStream *srtp.ReadStreamSRTCP
	context        TrackLocalContext

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
	sendCalled, stopCalled chan interface{}

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
		sendCalled: make(chan interface{}),
		stopCalled: make(chan interface{}),
		ssrc:       SSRC(randutil.NewMathRandomGenerator().Uint32()),
		id:         id,
	}
	r.interceptorRTCPReader = api.interceptor.BindRTCPReader(interceptor.RTCPReaderFunc(r.readRTCP))

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

	if r.hasSent() {
		if err := r.track.Unbind(r.context); err != nil {
			return err
		}
	}

	if !r.hasSent() || track == nil {
		r.track = track
		return nil
	}

	if _, err := track.Bind(r.context); err != nil {
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

	srtcpSession, err := r.transport.getSRTCPSession()
	if err != nil {
		return err
	}

	r.rtcpReadStream, err = srtcpSession.OpenReadStream(uint32(parameters.Encodings.SSRC))
	if err != nil {
		return err
	}

	srtpSession, err := r.transport.getSRTPSession()
	if err != nil {
		return err
	}

	rtpWriteStream, err := srtpSession.OpenWriteStream()
	if err != nil {
		return err
	}

	writeStream := &interceptorTrackLocalWriter{TrackLocalWriter: rtpWriteStream}

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
			interceptor.RTPWriterFunc(func(p *rtp.Packet, attributes interceptor.Attributes) (int, error) {
				return rtpWriteStream.WriteRTP(&p.Header, p.Payload)
			}),
		))

	close(r.sendCalled)
	return nil
}

// Stop irreversibly stops the RTPSender
func (r *RTPSender) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-r.stopCalled:
		return nil
	default:
	}
	close(r.stopCalled)

	if !r.hasSent() {
		return nil
	}

	return r.rtcpReadStream.Close()
}

// Read reads incoming RTCP for this RTPReceiver
func (r *RTPSender) Read(b []byte) (n int, err error) {
	select {
	case <-r.sendCalled:
		return r.rtcpReadStream.Read(b)
	case <-r.stopCalled:
		return 0, io.ErrClosedPipe
	}
}

// ReadRTCP is a convenience method that wraps Read and unmarshals for you.
// It also runs any configured interceptors.
func (r *RTPSender) ReadRTCP() ([]rtcp.Packet, error) {
	pkts, _, err := r.interceptorRTCPReader.Read()
	return pkts, err
}

func (r *RTPSender) readRTCP() ([]rtcp.Packet, interceptor.Attributes, error) {
	b := make([]byte, receiveMTU)
	i, err := r.Read(b)
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
