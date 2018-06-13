package webrtc

import (
	"fmt"
	"math/rand"

	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/ice"
	"github.com/pions/webrtc/internal/network"
	"github.com/pions/webrtc/internal/sdp"
	"github.com/pions/webrtc/pkg/rtp"

	"github.com/pkg/errors"
)

// MediaType determines the type of media we are sending receiving
type MediaType int

const (
	// G711 is a MediaType
	G711 MediaType = iota
	// G722 is a MediaType
	G722 MediaType = iota
	// ILBC is a MediaType
	ILBC MediaType = iota
	// ISAC is a MediaType
	ISAC MediaType = iota
	// H264 is a MediaType
	H264 MediaType = iota
	// VP8 is a MediaType
	VP8 MediaType = iota
	// Opus is a MediaType
	Opus MediaType = iota
)

// RTCPeerConnection represents a WebRTC connection between itself and a remote peer
type RTCPeerConnection struct {
	Ontrack          func(mediaType MediaType, buffers <-chan *rtp.Packet)
	LocalDescription *sdp.SessionDescription

	tlscfg *dtls.TLSCfg

	iceUsername string
	icePassword string
}

// Public

// SetRemoteDescription sets the SessionDescription of the remote peer
func (r *RTCPeerConnection) SetRemoteDescription(string) error {
	return nil
}

// CreateOffer starts the RTCPeerConnection and generates the localDescription
// The order of CreateOffer/SetRemoteDescription determines if we are the offerer or the answerer
// Once the RemoteDescription has been set network activity will start
func (r *RTCPeerConnection) CreateOffer() error {
	if r.tlscfg != nil {
		return errors.Errorf("tlscfg is already defined, CreateOffer can only be called once")
	}
	r.tlscfg = dtls.NewTLSCfg()
	r.iceUsername = randSeq(16)
	r.icePassword = randSeq(32)

	candidates := []string{}
	basePriority := uint16(rand.Uint32() & (1<<16 - 1))
	for id, c := range ice.HostInterfaces() {
		dstPort, err := network.UDPListener(c, []byte(r.icePassword), r.tlscfg, r.generateChannel)
		if err != nil {
			panic(err)
		}
		candidates = append(candidates, fmt.Sprintf("candidate:udpcandidate %d udp %d %s %d typ host", id, basePriority, c, dstPort))
		basePriority = basePriority + 1
	}

	r.LocalDescription = sdp.VP8OnlyDescription(r.iceUsername, r.icePassword, r.tlscfg.Fingerprint(), candidates)

	return nil
}

// AddStream adds a new media to the RTCPeerConnection
// This function returns a channel to push buffers on, and an error if the channel can't be added
// Closing the channel ends this stream
func (r *RTCPeerConnection) AddStream(mediaType MediaType) (buffers chan<- []byte, err error) {
	return nil, nil
}

// Private
func (r *RTCPeerConnection) generateChannel(ssrc uint32) (buffers chan<- *rtp.Packet) {
	if r.Ontrack == nil {
		return nil
	}

	bufferTransport := make(chan *rtp.Packet, 15)
	r.Ontrack(VP8, bufferTransport) // TODO look up media via SSRC in remote SD
	return bufferTransport
}
