// +build !js

package webrtc

import (
	"fmt"
	"io"
	"sync"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/srtp"
)

// RTPSender allows an application to control how a given Track is encoded and transmitted to a remote peer
type RTPSender struct {
	track          *Track
	rtcpReadStream *srtp.ReadStreamSRTCP

	transport *DTLSTransport

	// TODO(sgotti) remove this when in future we'll avoid replacing
	// a transceiver sender since we can just check the
	// transceiver negotiation status
	negotiated bool

	// A reference to the associated api object
	api *API

	mu                     sync.RWMutex
	sendCalled, stopCalled chan interface{}
}

// NewRTPSender constructs a new RTPSender
func (api *API) NewRTPSender(track *Track, transport *DTLSTransport) (*RTPSender, error) {
	if track == nil {
		return nil, fmt.Errorf("Track must not be nil")
	} else if transport == nil {
		return nil, fmt.Errorf("DTLSTransport must not be nil")
	}

	track.mu.Lock()
	defer track.mu.Unlock()
	if track.receiver != nil {
		return nil, fmt.Errorf("RTPSender can not be constructed with remote track")
	}
	track.totalSenderCount++

	return &RTPSender{
		track:      track,
		transport:  transport,
		api:        api,
		sendCalled: make(chan interface{}),
		stopCalled: make(chan interface{}),
	}, nil
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
func (r *RTPSender) Track() *Track {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.track
}

func (r *RTPSender) setTrack(track *Track) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.track = track
}

// Send Attempts to set the parameters controlling the sending of media.
func (r *RTPSender) Send(parameters RTPSendParameters) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.hasSent() {
		return fmt.Errorf("Send has already been called")
	}

	srtcpSession, err := r.transport.getSRTCPSession()
	if err != nil {
		return err
	}

	r.rtcpReadStream, err = srtcpSession.OpenReadStream(parameters.Encodings.SSRC)
	if err != nil {
		return err
	}

	r.track.mu.Lock()
	r.track.activeSenders = append(r.track.activeSenders, r)
	r.track.mu.Unlock()

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

	r.track.mu.Lock()
	defer r.track.mu.Unlock()
	filtered := []*RTPSender{}
	for _, s := range r.track.activeSenders {
		if s != r {
			filtered = append(filtered, s)
		} else {
			r.track.totalSenderCount--
		}
	}
	r.track.activeSenders = filtered
	close(r.stopCalled)

	if r.hasSent() {
		return r.rtcpReadStream.Close()
	}

	return nil
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

// ReadRTCP is a convenience method that wraps Read and unmarshals for you
func (r *RTPSender) ReadRTCP() ([]rtcp.Packet, error) {
	b := make([]byte, receiveMTU)
	i, err := r.Read(b)
	if err != nil {
		return nil, err
	}

	return rtcp.Unmarshal(b[:i])
}

// SendRTP sends a RTP packet on this RTPSender
//
// You should use Track instead to send packets. This is exposed because pion/webrtc currently
// provides no way for users to send RTP packets directly. This is makes users unable to send
// retransmissions to a single RTPSender. in /v3 this will go away, only use this API if you really
// need it.
func (r *RTPSender) SendRTP(header *rtp.Header, payload []byte) (int, error) {
	select {
	case <-r.stopCalled:
		return 0, fmt.Errorf("RTPSender has been stopped")
	case <-r.sendCalled:
		srtpSession, err := r.transport.getSRTPSession()
		if err != nil {
			return 0, err
		}

		writeStream, err := srtpSession.OpenWriteStream()
		if err != nil {
			return 0, err
		}

		return writeStream.WriteRTP(header, payload)
	}
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
