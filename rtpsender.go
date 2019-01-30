package webrtc

import (
	"fmt"
	"sync"

	"github.com/pions/rtcp"
	"github.com/pions/rtp"
	"github.com/pions/srtp"
	"github.com/pions/webrtc/pkg/media"
)

const rtpOutboundMTU = 1400

// RTPSender allows an application to control how a given Track is encoded and transmitted to a remote peer
type RTPSender struct {
	lock sync.RWMutex

	Track *Track

	transport *DTLSTransport

	// A reference to the associated api object
	api *API
}

// NewRTPSender constructs a new RTPSender
func (api *API) NewRTPSender(track *Track, transport *DTLSTransport) *RTPSender {
	return &RTPSender{
		Track:     track,
		transport: transport,
		api:       api,
	}
}

// Send Attempts to set the parameters controlling the sending of media.
func (r *RTPSender) Send(parameters RTPSendParameters) error {
	if r.Track.isRawRTP {
		go r.handleRawRTP(r.Track.rawInput)
	} else {
		go r.handleSampleRTP(r.Track.sampleInput)
	}

	dtls, err := r.Transport()
	if err != nil {
		return err
	}

	srtcpSession, err := dtls.getSrtcpSession()
	if err != nil {
		return err
	}

	ssrc := r.Track.SSRC
	srtcpStream, err := srtcpSession.OpenReadStream(ssrc)
	if err != nil {
		return fmt.Errorf("failed to open RTCP ReadStream, RTCTrack done for: %v %d", err, ssrc)
	}

	go r.handleRTCP(srtcpStream, r.Track.rtcpInput)

	return nil
}

// Stop irreversibly stops the RTPSender
func (r *RTPSender) Stop() {
	if r.Track.isRawRTP {
		close(r.Track.RawRTP)
	} else {
		close(r.Track.Samples)
	}

	// TODO properly tear down all loops (and test that)
}

func (r *RTPSender) handleRawRTP(rtpPackets chan *rtp.Packet) {
	for {
		p, ok := <-rtpPackets
		if !ok {
			return
		}

		err := r.sendRTP(p)
		if err != nil {
			pcLog.Warnf("failed to send RTP: %v", err)
		}
	}
}

func (r *RTPSender) handleSampleRTP(rtpPackets chan media.Sample) {
	packetizer := rtp.NewPacketizer(
		rtpOutboundMTU,
		r.Track.PayloadType,
		r.Track.SSRC,
		r.Track.Codec.Payloader,
		rtp.NewRandomSequencer(),
		r.Track.Codec.ClockRate,
	)

	for {
		in, ok := <-rtpPackets
		if !ok {
			return
		}
		packets := packetizer.Packetize(in.Data, in.Samples)
		for _, p := range packets {
			err := r.sendRTP(p)
			if err != nil {
				pcLog.Warnf("failed to send RTP: %v", err)
			}
		}
	}

}

// Transport returns the DTLSTransport instance over which
// RTP is sent and received.
func (r *RTPSender) Transport() (*DTLSTransport, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if r.transport == nil {
		return nil, fmt.Errorf("the DTLS transport is not started")
	}

	return r.transport, nil
}

func (r *RTPSender) handleRTCP(stream *srtp.ReadStreamSRTCP, rtcpPackets chan rtcp.Packet) {
	var rtcpPacket rtcp.Packet
	for {
		rtcpBuf := make([]byte, receiveMTU)
		i, err := stream.Read(rtcpBuf)
		if err != nil {
			pcLog.Warnf("Failed to read, Track done for: %v %d \n", err, r.Track.SSRC)
			return
		}

		rtcpPacket, _, err = rtcp.Unmarshal(rtcpBuf[:i])
		if err != nil {
			pcLog.Warnf("Failed to unmarshal RTCP packet, discarding: %v \n", err)
			continue
		}

		select {
		case rtcpPackets <- rtcpPacket:
		default:
		}
	}
}

func (r *RTPSender) sendRTP(packet *rtp.Packet) error {
	dtls, err := r.Transport()
	if err != nil {
		return err
	}

	srtpSession, err := dtls.getSrtpSession()
	if err != nil {
		return err
	}

	writeStream, err := srtpSession.OpenWriteStream()
	if err != nil {
		return fmt.Errorf("failed to open WriteStream: %v", err)
	}

	if _, err := writeStream.WriteRTP(&packet.Header, packet.Payload); err != nil {
		return fmt.Errorf("failed to write: %v", err)
	}

	return nil
}
