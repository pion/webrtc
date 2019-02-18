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

	hasRecv chan bool

	Track *Track

	closed bool
	mu     sync.Mutex

	rtpOut        chan *rtp.Packet
	rtpReadStream *srtp.ReadStreamSRTP
	rtpOutDone    chan bool

	rtcpOut        chan rtcp.Packet
	rtcpReadStream *srtp.ReadStreamSRTCP
	rtcpOutDone    chan bool
}

// NewRTPReceiver constructs a new RTPReceiver
func NewRTPReceiver(kind RTPCodecType, transport *DTLSTransport) *RTPReceiver {
	return &RTPReceiver{
		kind:      kind,
		transport: transport,

		rtpOut:     make(chan *rtp.Packet, 15),
		rtpOutDone: make(chan bool),

		rtcpOut:     make(chan rtcp.Packet, 15),
		rtcpOutDone: make(chan bool),

		hasRecv: make(chan bool),
	}
}

// Receive blocks until the Track is available
func (r *RTPReceiver) Receive(parameters RTPReceiveParameters) chan bool {
	// TODO atomic only allow this to fire once
	r.Track = &Track{
		Kind:        r.kind,
		SSRC:        parameters.encodings.SSRC,
		Packets:     r.rtpOut,
		RTCPPackets: r.rtcpOut,
	}

	// RTP ReadLoop
	go func() {
		payloadSet := false
		defer func() {
			if !payloadSet {
				close(r.hasRecv)
			}
			close(r.rtpOut)
			close(r.rtpOutDone)
		}()

		srtpSession, err := r.transport.getSRTPSession()
		if err != nil {
			pcLog.Warnf("Failed to open SRTPSession, Track done for: %v %d \n", err, parameters.encodings.SSRC)
			return
		}

		readStream, err := srtpSession.OpenReadStream(parameters.encodings.SSRC)
		if err != nil {
			pcLog.Warnf("Failed to open RTCP ReadStream, Track done for: %v %d \n", err, parameters.encodings.SSRC)
			return
		}
		r.mu.Lock()
		r.rtpReadStream = readStream
		r.mu.Unlock()

		readBuf := make([]byte, receiveMTU)
		for {
			rtpLen, err := readStream.Read(readBuf)
			if err != nil {
				pcLog.Warnf("Failed to read, Track done for: %v %d \n", err, parameters.encodings.SSRC)
				return
			}

			var rtpPacket rtp.Packet
			if err = rtpPacket.Unmarshal(append([]byte{}, readBuf[:rtpLen]...)); err != nil {
				pcLog.Warnf("Failed to unmarshal RTP packet, discarding: %v \n", err)
				continue
			}

			if !payloadSet {
				r.Track.PayloadType = rtpPacket.PayloadType
				payloadSet = true
				close(r.hasRecv)
			}

			select {
			case r.rtpOut <- &rtpPacket:
			default:
			}
		}
	}()

	// RTCP ReadLoop
	go func() {
		defer func() {
			close(r.rtcpOut)
			close(r.rtcpOutDone)
		}()

		srtcpSession, err := r.transport.getSRTCPSession()
		if err != nil {
			pcLog.Warnf("Failed to open SRTCPSession, Track done for: %v %d \n", err, parameters.encodings.SSRC)
			return
		}

		readStream, err := srtcpSession.OpenReadStream(parameters.encodings.SSRC)
		if err != nil {
			pcLog.Warnf("Failed to open RTCP ReadStream, Track done for: %v %d \n", err, parameters.encodings.SSRC)
			return
		}
		r.mu.Lock()
		r.rtcpReadStream = readStream
		r.mu.Unlock()

		readBuf := make([]byte, receiveMTU)
		for {
			rtcpLen, err := readStream.Read(readBuf)
			if err != nil {
				pcLog.Warnf("Failed to read, Track done for: %v %d \n", err, parameters.encodings.SSRC)
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
	}()

	return r.hasRecv
}

// Stop irreversibly stops the RTPReceiver
func (r *RTPReceiver) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return fmt.Errorf("RTPReceiver has already been closed")
	}

	select {
	case <-r.hasRecv:
	default:
		return fmt.Errorf("RTPReceiver has not been started")
	}

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
