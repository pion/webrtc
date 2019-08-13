// +build !js

package webrtc

import (
	"fmt"
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

	// A reference to the associated api object
	api *API

	mu                     sync.RWMutex
	sendCalled, stopCalled chan interface{}

	payloadType *uint8 // Senders should have a codec parameter dictionary at some point
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
	track.totalSenderCount++

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
	<-r.sendCalled
	return r.rtcpReadStream.Read(b)
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

// sendRTP should only be called by a track, this only exists so we can keep state in one place.
// Overwrites the payload type field in the rtp header.
func (r *RTPSender) sendRTP(header *rtp.Header, payload []byte) (int, error) {
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
		// Hopefully this next part is temporary and will be removed when senders obtain payload
		// types from their session instead of the track.
		// Obtain payload type for this sender. Currently taken from the sender's MediaEngine
		// to match the track's codec, which could have a different payload type.
		// (But tracks should not have codecs - this should be set here by the
		// peer connection or transceiver...)
		if r.payloadType == nil {
			// this setup should only happen on the first call to sendRTP
			codecs := r.api.mediaEngine.GetCodecsByName(r.track.codec.Name)
			if len(codecs) == 0 {
				return 0, fmt.Errorf("no %s codecs in media engine", r.track.codec.Name)
			}
			for _,c := range codecs {
				if sameCodec(c, r.track.codec) {
					r.payloadType = &c.PayloadType
					break
				}
			}
			if r.payloadType == nil {
				return 0, fmt.Errorf("could not match %s codec from track to media engine", r.track.codec.Name)
			}
		}
		// Overwrite the payload type in the RTP header.
		if r.payloadType != nil {
			header.PayloadType = *r.payloadType
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


// sameCodec indicates if two codecs match in type, parameters,
// etc, not checking payload type, so it is useful for comparing
// codecs from different MediaEngines
func sameCodec(codecA, codecB *RTPCodec) bool {
	if codecA.Name != codecB.Name {
		return false
	}
	if codecA.Type != codecB.Type {
		return false
	}
	if codecA.SDPFmtpLine != codecB.SDPFmtpLine {
		return false
	}
	if codecA.Channels != codecB.Channels {
		return false
	}
	if codecA.ClockRate != codecB.ClockRate {
		return false
	}
	if codecA.MimeType != codecB.MimeType {
		return false
	}
	return true
}