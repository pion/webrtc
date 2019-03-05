package webrtc

import (
	"fmt"
	"sync"

	"github.com/pions/rtcp"
)

// RTPSender allows an application to control how a given Track is encoded and transmitted to a remote peer
type RTPSender struct {
	track          *Track
	rtcpReadStream *lossyReadCloser

	transport *DTLSTransport

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

	track.mu.RLock()
	defer track.mu.RUnlock()
	if track.receiver != nil {
		return nil, fmt.Errorf("RTPSender can not be constructed with remote track")
	}

	return &RTPSender{
		track:      track,
		transport:  transport,
		api:        api,
		sendCalled: make(chan interface{}),
		stopCalled: make(chan interface{}),
	}, nil
}

// Transport returns the currently-configured *DTLSTransport or nil
// if one has not yet been configured
func (r *RTPSender) Transport() *DTLSTransport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.transport
}

// Send Attempts to set the parameters controlling the sending of media.
func (r *RTPSender) Send(parameters RTPSendParameters) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-r.sendCalled:
		return fmt.Errorf("Send has already been called")
	default:
	}

	srtcpSession, err := r.transport.getSRTCPSession()
	if err != nil {
		return err
	}
	srtcpReadStream, err := srtcpSession.OpenReadStream(parameters.Encodings.SSRC)
	if err != nil {
		return err
	}
	r.rtcpReadStream = newLossyReadCloser(srtcpReadStream)

	r.track.mu.Lock()
	r.track.senders = append(r.track.senders, r)
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
	for _, s := range r.track.senders {
		if s != r {
			filtered = append(filtered, s)
		}
	}
	r.track.senders = filtered

	select {
	case <-r.sendCalled:
		return r.rtcpReadStream.Close()
	default:
	}

	close(r.stopCalled)
	return nil
}

// Read reads incoming RTCP for this RTPReceiver
func (r *RTPSender) Read(b []byte) (n int, err error) {
	select {
	case <-r.stopCalled:
		return 0, fmt.Errorf("RTPSender has been stopped")
	case <-r.sendCalled:
		return r.rtcpReadStream.Read(b)
	}
}

// ReadRTCP is a convenience method that wraps Read and unmarshals for you
func (r *RTPSender) ReadRTCP() (rtcp.Packet, error) {
	b := make([]byte, receiveMTU)
	i, err := r.Read(b)
	if err != nil {
		return nil, err
	}

	pkt, _, err := rtcp.Unmarshal(b[:i])
	return pkt, err
}

// sendRTP should only be called by a track, this only exists so we can keep state in one place
func (r *RTPSender) sendRTP(b []byte) (int, error) {
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

		return writeStream.Write(b)
	}
}
