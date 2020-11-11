// +build !js

package webrtc

import (
	"fmt"
	"io"
	"sync"

	"github.com/pion/rtcp"
	"github.com/pion/srtp"
)

// trackStreams maintains a mapping of RTP/RTCP streams to a specific track
// a RTPReceiver may contain multiple streams if we are dealing with Multicast
type trackStreams struct {
	track          *TrackRemote
	rtpReadStream  *srtp.ReadStreamSRTP
	rtcpReadStream *srtp.ReadStreamSRTCP
}

// RTPReceiver allows an application to inspect the receipt of a TrackRemote
type RTPReceiver struct {
	kind      RTPCodecType
	transport *DTLSTransport

	tracks []trackStreams

	closed, received chan interface{}
	mu               sync.RWMutex

	// A reference to the associated api object
	api *API
}

// NewRTPReceiver constructs a new RTPReceiver
func (api *API) NewRTPReceiver(kind RTPCodecType, transport *DTLSTransport) (*RTPReceiver, error) {
	if transport == nil {
		return nil, errRTPReceiverDTLSTransportNil
	}

	return &RTPReceiver{
		kind:      kind,
		transport: transport,
		api:       api,
		closed:    make(chan interface{}),
		received:  make(chan interface{}),
		tracks:    []trackStreams{},
	}, nil
}

// Transport returns the currently-configured *DTLSTransport or nil
// if one has not yet been configured
func (r *RTPReceiver) Transport() *DTLSTransport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.transport
}

// Track returns the RtpTransceiver TrackRemote
func (r *RTPReceiver) Track() *TrackRemote {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.tracks) != 1 {
		return nil
	}
	return r.tracks[0].track
}

// Tracks returns the RtpTransceiver tracks
// A RTPReceiver to support Simulcast may now have multiple tracks
func (r *RTPReceiver) Tracks() []*TrackRemote {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tracks []*TrackRemote
	for i := range r.tracks {
		tracks = append(tracks, r.tracks[i].track)
	}
	return tracks
}

// Receive initialize the track and starts all the transports
func (r *RTPReceiver) Receive(parameters RTPReceiveParameters) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-r.received:
		return errRTPReceiverReceiveAlreadyCalled
	default:
	}
	defer close(r.received)

	if len(parameters.Encodings) == 1 && parameters.Encodings[0].SSRC != 0 {
		t := trackStreams{
			track: &TrackRemote{
				kind:     r.kind,
				ssrc:     parameters.Encodings[0].SSRC,
				receiver: r,
			},
		}

		var err error
		t.rtpReadStream, t.rtcpReadStream, err = r.streamsForSSRC(parameters.Encodings[0].SSRC)
		if err != nil {
			return err
		}

		r.tracks = append(r.tracks, t)
	} else {
		for _, encoding := range parameters.Encodings {
			r.tracks = append(r.tracks, trackStreams{
				track: &TrackRemote{
					kind:     r.kind,
					rid:      encoding.RID,
					receiver: r,
				},
			})
		}
	}

	return nil
}

// Read reads incoming RTCP for this RTPReceiver
func (r *RTPReceiver) Read(b []byte) (n int, err error) {
	select {
	case <-r.received:
		return r.tracks[0].rtcpReadStream.Read(b)
	case <-r.closed:
		return 0, io.ErrClosedPipe
	}
}

// ReadSimulcast reads incoming RTCP for this RTPReceiver for given rid
func (r *RTPReceiver) ReadSimulcast(b []byte, rid string) (n int, err error) {
	select {
	case <-r.received:
		for _, t := range r.tracks {
			if t.track != nil && t.track.rid == rid {
				return t.rtcpReadStream.Read(b)
			}
		}
		return 0, fmt.Errorf("%w: %s", errRTPReceiverForRIDTrackStreamNotFound, rid)
	case <-r.closed:
		return 0, io.ErrClosedPipe
	}
}

// ReadRTCP is a convenience method that wraps Read and unmarshal for you
func (r *RTPReceiver) ReadRTCP() ([]rtcp.Packet, error) {
	b := make([]byte, receiveMTU)
	i, err := r.Read(b)
	if err != nil {
		return nil, err
	}

	return rtcp.Unmarshal(b[:i])
}

// ReadSimulcastRTCP is a convenience method that wraps ReadSimulcast and unmarshal for you
func (r *RTPReceiver) ReadSimulcastRTCP(rid string) ([]rtcp.Packet, error) {
	b := make([]byte, receiveMTU)
	i, err := r.ReadSimulcast(b, rid)
	if err != nil {
		return nil, err
	}

	return rtcp.Unmarshal(b[:i])
}

func (r *RTPReceiver) haveReceived() bool {
	select {
	case <-r.received:
		return true
	default:
		return false
	}
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
		for i := range r.tracks {
			if r.tracks[i].rtcpReadStream != nil {
				if err := r.tracks[i].rtcpReadStream.Close(); err != nil {
					return err
				}
			}
			if r.tracks[i].rtpReadStream != nil {
				if err := r.tracks[i].rtpReadStream.Close(); err != nil {
					return err
				}
			}
		}
	default:
	}

	close(r.closed)
	return nil
}

func (r *RTPReceiver) streamsForTrack(t *TrackRemote) *trackStreams {
	for i := range r.tracks {
		if r.tracks[i].track == t {
			return &r.tracks[i]
		}
	}
	return nil
}

// readRTP should only be called by a track, this only exists so we can keep state in one place
func (r *RTPReceiver) readRTP(b []byte, reader *TrackRemote) (n int, err error) {
	<-r.received
	if t := r.streamsForTrack(reader); t != nil {
		return t.rtpReadStream.Read(b)
	}

	return 0, fmt.Errorf("%w: %d", errRTPReceiverWithSSRCTrackStreamNotFound, reader.SSRC())
}

// receiveForRid is the sibling of Receive expect for RIDs instead of SSRCs
// It populates all the internal state for the given RID
func (r *RTPReceiver) receiveForRid(rid string, codec RTPCodecParameters, ssrc SSRC) (*TrackRemote, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.tracks {
		if r.tracks[i].track.RID() == rid {
			r.tracks[i].track.mu.Lock()
			r.tracks[i].track.kind = r.kind
			r.tracks[i].track.codec = codec
			r.tracks[i].track.ssrc = ssrc
			r.tracks[i].track.mu.Unlock()

			var err error
			r.tracks[i].rtpReadStream, r.tracks[i].rtcpReadStream, err = r.streamsForSSRC(ssrc)
			if err != nil {
				return nil, err
			}

			return r.tracks[i].track, nil
		}
	}

	return nil, fmt.Errorf("%w: %d", errRTPReceiverForSSRCTrackStreamNotFound, ssrc)
}

func (r *RTPReceiver) streamsForSSRC(ssrc SSRC) (*srtp.ReadStreamSRTP, *srtp.ReadStreamSRTCP, error) {
	srtpSession, err := r.transport.getSRTPSession()
	if err != nil {
		return nil, nil, err
	}

	rtpReadStream, err := srtpSession.OpenReadStream(uint32(ssrc))
	if err != nil {
		return nil, nil, err
	}

	srtcpSession, err := r.transport.getSRTCPSession()
	if err != nil {
		return nil, nil, err
	}

	rtcpReadStream, err := srtcpSession.OpenReadStream(uint32(ssrc))
	if err != nil {
		return nil, nil, err
	}

	return rtpReadStream, rtcpReadStream, nil
}
