package webrtc

import (
	"fmt"
	"math/rand"

	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/ice"
	"github.com/pions/webrtc/internal/network"
	"github.com/pions/webrtc/internal/sdp"

	"github.com/pkg/errors"
)

type MediaType int

const (
	G711 MediaType = iota
	G722 MediaType = iota
	ILBC MediaType = iota
	ISAC MediaType = iota
	H264 MediaType = iota
	VP8  MediaType = iota
	Opus MediaType = iota
)

type RTCPeerConnection struct {
	Ontrack          func(mediaType MediaType, buffers <-chan []byte)
	LocalDescription *sdp.SessionDescription

	tlscfg *dtls.TLSCfg

	iceUsername string
	icePassword string
}

// Public

func (r *RTCPeerConnection) SetRemoteDescription(string) error {
	return nil
}

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
		dstPort, err := network.UdpListener(c, []byte(r.icePassword), r.tlscfg)
		if err != nil {
			panic(err)
		}
		candidates = append(candidates, fmt.Sprintf("candidate:udpcandidate %d udp %d %s %d typ host", id, basePriority, c, dstPort))
		basePriority = basePriority + 1
	}

	r.LocalDescription = sdp.VP8OnlyDescription(r.iceUsername, r.icePassword, r.tlscfg.Fingerprint(), candidates)

	return nil
}

func (r *RTCPeerConnection) AddStream(mediaType MediaType) (buffers chan<- []byte, err error) {
	return nil, nil
}

// Private
