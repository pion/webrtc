package webrtc

import (
	"fmt"
	"sync"

	"github.com/pions/rtcp"
	"github.com/pions/rtp"
	"github.com/pions/srtp"
)

// RTPReceiver allows an application to inspect the receipt of a Track
type RTPReceiver struct {
	kind      RTPCodecType
	transport *DTLSTransport

	Track *Track

	closed bool
	mu     sync.Mutex

	rtpOut        chan *rtp.Packet
	rtpReadStream *srtp.ReadStreamSRTP
	rtpOutDone    chan struct{}

	rtcpOut        chan rtcp.Packet
	rtcpReadStream *srtp.ReadStreamSRTCP
	rtcpOutDone    chan struct{}

	// A reference to the associated api object
	api *API
}

// NewRTPReceiver constructs a new RTPReceiver
func (api *API) NewRTPReceiver(kind RTPCodecType, transport *DTLSTransport) *RTPReceiver {
	return &RTPReceiver{
		kind:      kind,
		transport: transport,

		rtpOut:     make(chan *rtp.Packet, 15),
		rtpOutDone: make(chan struct{}),

		rtcpOut:     make(chan rtcp.Packet, 15),
		rtcpOutDone: make(chan struct{}),

		api: api,
	}
}

// Receive blocks until the RTCTrack is available
func (r *RTPReceiver) Receive(parameters RTPReceiveParameters) error {
	// TODO atomic only allow this to fire once
	ssrc := parameters.Encodings[0].SSRC

	srtpSession := r.transport.srtpSession
	readStreamRTP, err := srtpSession.OpenReadStream(ssrc)
	if err != nil {
		return fmt.Errorf("failed to open RTP ReadStream %d: %v", ssrc, err)
	}

	srtcpSession := r.transport.srtcpSession
	readStreamRTCP, err := srtcpSession.OpenReadStream(ssrc)
	if err != nil {
		return fmt.Errorf("failed to open RTCP ReadStream %d: %v", ssrc, err)
	}

	// Start readloops
	recvLoopRTP, payloadTypeCh := r.createRecvLoopRTP()

	go recvLoopRTP(readStreamRTP, ssrc)
	go r.recvLoopRTCP(readStreamRTCP, ssrc)

	payloadType := <-payloadTypeCh
	codecParams, err := parameters.getCodecParameters(payloadType)
	if err != nil {
		return fmt.Errorf("failed to find codec parameters: %v", err)
	}

	codec, err := r.api.mediaEngine.getCodecSDP(codecParams)
	if err != nil {
		return fmt.Errorf("codec %s is not registered", codecParams)
	}

	// Set the receiver track
	r.Track = &Track{
		PayloadType: payloadType,
		Kind:        codec.Type,
		Codec:       codec,
		SSRC:        ssrc,
		Packets:     r.rtpOut,
		RTCPPackets: r.rtcpOut,
	}

	return nil
}

func (r *RTPReceiver) createRecvLoopRTP() (func(stream *srtp.ReadStreamSRTP, ssrc uint32), chan uint8) {
	payloadTypeCh := make(chan uint8)
	return func(stream *srtp.ReadStreamSRTP, ssrc uint32) {
		r.mu.Lock()
		r.rtpReadStream = stream
		r.mu.Unlock()

		defer func() {
			close(r.rtpOut)
			close(r.rtpOutDone)
		}()
		readBuf := make([]byte, receiveMTU)
		for {
			rtpLen, err := stream.Read(readBuf)
			if err != nil {
				pcLog.Warnf("Failed to read, Track done for: %v %d \n", err, ssrc)
				return
			}

			var rtpPacket rtp.Packet
			if err = rtpPacket.Unmarshal(append([]byte{}, readBuf[:rtpLen]...)); err != nil {
				pcLog.Warnf("Failed to unmarshal RTP packet, discarding: %v \n", err)
				continue
			}

			select {
			case payloadTypeCh <- rtpPacket.PayloadType:
				payloadTypeCh = nil
			case r.rtpOut <- &rtpPacket:
			default:
			}
		}
	}, payloadTypeCh
}

func (r *RTPReceiver) recvLoopRTCP(stream *srtp.ReadStreamSRTCP, ssrc uint32) {
	r.mu.Lock()
	r.rtcpReadStream = stream
	r.mu.Unlock()

	defer func() {
		close(r.rtcpOut)
		close(r.rtcpOutDone)
	}()
	readBuf := make([]byte, receiveMTU)
	for {
		rtcpLen, err := stream.Read(readBuf)
		if err != nil {
			pcLog.Warnf("Failed to read, Track done for: %v %d \n", err, ssrc)
			return
		}

		rtcpPacket, _, err := rtcp.Unmarshal(append([]byte{}, readBuf[:rtcpLen]...))
		if err != nil {
			pcLog.Warnf("Failed to unmarshal RTCP packet, discarding: %v \n", err)
			continue
		}

		select {
		case r.rtcpOut <- rtcpPacket:
		default:
		}
	}
}

// Stop irreversibly stops the RTPReceiver
func (r *RTPReceiver) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return fmt.Errorf("RTPReceiver has already been closed")
	}

	fmt.Println("Closing receiver")
	if err := r.rtcpReadStream.Close(); err != nil {
		return err
	}
	if err := r.rtpReadStream.Close(); err != nil {
		return err
	}

	<-r.rtcpOutDone
	<-r.rtpOutDone

	r.closed = true
	return nil
}
