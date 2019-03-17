// +build !js

package webrtc

import (
	"fmt"
	"sync"

	"github.com/pions/rtcp"
	"github.com/pions/srtp"
)

// RTPReceiver allows an application to inspect the receipt of a Track
type RTPReceiver struct {
	kind      RTPCodecType
	transport *DTLSTransport

	track *Track

	closed, received chan interface{}
	mu               sync.RWMutex

	rtpReadStream  *srtp.ReadStreamSRTP
	rtcpReadStream *srtp.ReadStreamSRTCP

	// A reference to the associated api object
	api *API
}

// NewRTPReceiver constructs a new RTPReceiver
func (api *API) NewRTPReceiver(kind RTPCodecType, transport *DTLSTransport) (*RTPReceiver, error) {
	if transport == nil {
		return nil, fmt.Errorf("DTLSTransport must not be nil")
	}

	return &RTPReceiver{
		kind:      kind,
		transport: transport,
		api:       api,
		closed:    make(chan interface{}),
		received:  make(chan interface{}),
	}, nil
}

// Transport returns the currently-configured *DTLSTransport or nil
// if one has not yet been configured
func (r *RTPReceiver) Transport() *DTLSTransport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.transport
}

// Track returns the RTCRtpTransceiver track
func (r *RTPReceiver) Track() *Track {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.track
}

// Receive initialize the track and starts all the transports
func (r *RTPReceiver) Receive(parameters RTPReceiveParameters) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-r.received:
		return fmt.Errorf("Receive has already been called")
	default:
	}
	close(r.received)

	r.track = &Track{
		kind:     r.kind,
		ssrc:     parameters.Encodings.SSRC,
		receiver: r,
	}

	srtpSession, err := r.transport.getSRTPSession()
	if err != nil {
		return err
	}

	r.rtpReadStream, err = srtpSession.OpenReadStream(parameters.Encodings.SSRC)
	if err != nil {
		return err
	}

	srtcpSession, err := r.transport.getSRTCPSession()
	if err != nil {
		return err
	}

	r.rtcpReadStream, err = srtcpSession.OpenReadStream(parameters.Encodings.SSRC)
	if err != nil {
		return err
	}

	return nil
}

// Read reads incoming RTCP for this RTPReceiver
func (r *RTPReceiver) Read(b []byte) (n int, err error) {
	select {
	case <-r.closed:
		return 0, fmt.Errorf("RTPSender has been stopped")
	case <-r.received:
		return r.rtcpReadStream.Read(b)
	}
}

// ReadRTCP is a convenience method that wraps Read and unmarshals for you
func (r *RTPReceiver) ReadRTCP() (rtcp.CompoundPacket, error) {
	b := make([]byte, receiveMTU)
	i, err := r.Read(b)
	if err != nil {
		return nil, err
	}

	return rtcp.Unmarshal(b[:i])
}

// Stop irreversibly stops the RTPReceiver
func (r *RTPReceiver) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-r.closed:
		return nil
	default:
	}

	select {
	case <-r.received:
		if err := r.rtcpReadStream.Close(); err != nil {
			return err
		}
		if err := r.rtpReadStream.Close(); err != nil {
			return err
		}
	default:
	}

	close(r.closed)
	return nil
}

// readRTP should only be called by a track, this only exists so we can keep state in one place
func (r *RTPReceiver) readRTP(b []byte) (n int, err error) {
	select {
	case <-r.closed:
		return 0, fmt.Errorf("RTPSender has been stopped")
	case <-r.received:
		return r.rtpReadStream.Read(b)
	}
}
