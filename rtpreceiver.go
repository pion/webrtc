// +build !js

package webrtc

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/pion/rtcp"
	"github.com/pion/srtp"
)

// RTPReceiver allows an application to inspect the receipt of a Track
type RTPReceiver struct {
	kind      RTPCodecType
	transport *DTLSTransport

	track *Track

	closed, received chan interface{}
	mu               sync.RWMutex

	useRid      bool
	multiStream bool

	// streamsIndex contains the stream index by streamID (stream ssrc or rid)
	streamsIndex map[string]int

	// since the number of streams is fixed and known we use a precreated slice with the known size
	// and we access the stream by their id using the above ridsIndex
	// using a map will require locking the map for every call to Read
	rtpReadStreams       []*srtp.ReadStreamSRTP
	rtcpReadStreams      []*srtp.ReadStreamSRTCP
	rtpReadStreamsReady  []chan struct{}
	rtcpReadStreamsReady []chan struct{}

	// A reference to the associated api object
	api *API
}

// NewRTPReceiver constructs a new RTPReceiver
func (api *API) NewRTPReceiver(kind RTPCodecType, transport *DTLSTransport) (*RTPReceiver, error) {
	if transport == nil {
		return nil, fmt.Errorf("DTLSTransport must not be nil")
	}

	return &RTPReceiver{
		kind:         kind,
		transport:    transport,
		api:          api,
		streamsIndex: make(map[string]int),
		closed:       make(chan interface{}),
		received:     make(chan interface{}),
	}, nil
}

func (r *RTPReceiver) readyStreams() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c := 0
	for _, s := range r.rtpReadStreams {
		if s != nil {
			c++
		}
	}
	return c
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

	if len(parameters.Encodings) == 0 {
		return fmt.Errorf("no encodings provided")
	}
	select {
	case <-r.received:
		return fmt.Errorf("Receive has already been called")
	default:
	}
	defer close(r.received)

	r.track = &Track{
		kind:        r.kind,
		streams:     make([]*TrackRTPStream, len(parameters.Encodings)),
		receiver:    r,
		multiStream: len(parameters.Encodings) > 1,
	}

	r.rtpReadStreams = make([]*srtp.ReadStreamSRTP, len(parameters.Encodings))
	r.rtcpReadStreams = make([]*srtp.ReadStreamSRTCP, len(parameters.Encodings))
	r.rtpReadStreamsReady = make([]chan struct{}, len(parameters.Encodings))
	r.rtcpReadStreamsReady = make([]chan struct{}, len(parameters.Encodings))

	for i, enc := range parameters.Encodings {
		// use the ssrc (since it's fixed) as the stream index
		streamID := strconv.FormatUint(uint64(enc.SSRC), 10)
		if r.useRid {
			if enc.RID == "" {
				return fmt.Errorf("receiver is rid based but encoding doesn't have a rid")
			}
			streamID = enc.RID
		}

		r.track.streams[i] = &TrackRTPStream{
			id:    streamID,
			rid:   enc.RID,
			ssrc:  enc.SSRC,
			track: r.track,
		}

		r.streamsIndex[streamID] = i
		r.rtpReadStreamsReady[i] = make(chan struct{})
		r.rtcpReadStreamsReady[i] = make(chan struct{})
	}

	// whe not using rids we already know the stream ssrc so we can setup it here
	if !r.useRid {
		srtpSession, err := r.transport.getSRTPSession()
		if err != nil {
			return err
		}

		r.rtpReadStreams[0], err = srtpSession.OpenReadStream(parameters.Encodings[0].SSRC)
		if err != nil {
			return err
		}

		srtcpSession, err := r.transport.getSRTCPSession()
		if err != nil {
			return err
		}

		r.rtcpReadStreams[0], err = srtcpSession.OpenReadStream(parameters.Encodings[0].SSRC)
		if err != nil {
			return err
		}

		r.track.streams[0].ready = true

		close(r.rtpReadStreamsReady[0])
		close(r.rtcpReadStreamsReady[0])
	}

	return nil
}

// setRTPReadStream sets a rtpReadStream. The stream index is the rid if the receiver is rid based or the ssrc if not rid based
func (r *RTPReceiver) setRTPReadStream(rs *srtp.ReadStreamSRTP, rid string, ssrc uint32, payloadType uint8, codec *RTPCodec) {
	<-r.received

	r.mu.Lock()
	defer r.mu.Unlock()

	streamID := strconv.FormatUint(uint64(ssrc), 10)
	if r.useRid {
		streamID = rid
	}

	idx := r.streamsIndex[streamID]
	if r.rtpReadStreams[idx] != nil {
		return
	}

	r.rtpReadStreams[idx] = rs

	// open a rtcp read stream for the same ssrc
	srtcpSession, _ := r.transport.getSRTCPSession()
	r.rtcpReadStreams[idx], _ = srtcpSession.OpenReadStream(ssrc)

	close(r.rtpReadStreamsReady[idx])
	close(r.rtcpReadStreamsReady[idx])

	r.track.mu.Lock()
	r.track.streams[idx].mu.Lock()
	r.track.streams[idx].ready = true
	r.track.streams[idx].ssrc = ssrc
	r.track.streams[idx].mu.Unlock()

	// Set the same payload for all streams
	// TODO(sgotti) handle different payloaf for streams in the same track? Currently no implementation have different payloads
	for _, stream := range r.track.streams {
		stream.mu.Lock()
		stream.payloadType = payloadType
		stream.codec = codec
		stream.mu.Unlock()
	}
	r.track.mu.Unlock()
}

// Read reads incoming RTCP for this RTPReceiver
// If this receiver is multistream it'll return an error (use ReadStreamID)
func (r *RTPReceiver) Read(b []byte) (n int, err error) {
	if r.multiStream {
		return 0, fmt.Errorf("receiver is multistream")
	}

	<-r.rtcpReadStreamsReady[0]
	return r.rtcpReadStreams[0].Read(b)
}

// ReadStreamID reads incoming RTCP for this RTPReceiver
func (r *RTPReceiver) ReadStreamID(b []byte, streamID string) (n int, err error) {
	// TODO(sgotti) implement replaceable read streams (when ssrc for a rid changes)
	idx := r.streamsIndex[streamID]

	<-r.rtpReadStreamsReady[idx]
	return r.rtpReadStreams[idx].Read(b)
}

// ReadRTCP is a convenience method that wraps Read and unmarshals for you
func (r *RTPReceiver) ReadRTCP() ([]rtcp.Packet, error) {
	if r.multiStream {
		return nil, fmt.Errorf("receiver is multistream")
	}

	<-r.rtcpReadStreamsReady[0]

	b := make([]byte, receiveMTU)
	i, err := r.rtcpReadStreams[0].Read(b)
	if err != nil {
		return nil, err
	}

	return rtcp.Unmarshal(b[:i])
}

// ReadRTCPStreamID is a convenience method that wraps Read and unmarshals for you
func (r *RTPReceiver) ReadRTCPStreamID(streamID string) ([]rtcp.Packet, error) {
	idx := r.streamsIndex[streamID]

	<-r.rtpReadStreamsReady[idx]

	b := make([]byte, receiveMTU)
	i, err := r.rtcpReadStreams[idx].Read(b)
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

	for _, s := range r.rtpReadStreams {
		if s != nil {
			if err := s.Close(); err != nil {
				return err
			}
		}
	}

	for _, s := range r.rtcpReadStreams {
		if s != nil {
			if err := s.Close(); err != nil {
				return err
			}
		}
	}

	close(r.closed)
	return nil
}

func (r *RTPReceiver) readRTPStreamID(b []byte, streamID string) (n int, err error) {
	// TODO(sgotti) implement replaceable read streams (when ssrc for a rid changes)
	idx := r.streamsIndex[streamID]

	<-r.rtpReadStreamsReady[idx]
	return r.rtpReadStreams[idx].Read(b)
}
