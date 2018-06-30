package webrtc

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/network"
	"github.com/pions/webrtc/internal/sdp"
	"github.com/pions/webrtc/internal/util"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtp"

	"github.com/pkg/errors"
)

// TrackType determines the type of media we are sending receiving
type TrackType int

// List of supported TrackTypes
const (
	G711 TrackType = iota
	G722
	ILBC
	ISAC
	H264
	VP8
	Opus
)

// RTCPeerConnection represents a WebRTC connection between itself and a remote peer
type RTCPeerConnection struct {
	Ontrack                    func(mediaType TrackType, buffers <-chan *rtp.Packet)
	LocalDescription           *sdp.SessionDescription
	OnICEConnectionStateChange func(iceConnectionState ice.ConnectionState)

	tlscfg *dtls.TLSCfg

	iceUsername string
	icePassword string
	iceState    ice.ConnectionState

	portsLock sync.RWMutex
	ports     []*network.Port
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
	r.iceUsername = util.RandSeq(16)
	r.icePassword = util.RandSeq(32)

	r.portsLock.Lock()
	defer r.portsLock.Unlock()

	candidates := []string{}
	basePriority := uint16(rand.Uint32() & (1<<16 - 1))
	for id, c := range ice.HostInterfaces() {
		port, err := network.NewPort(c+":0", []byte(r.icePassword), r.tlscfg, r.generateChannel, r.iceStateChange)
		if err != nil {
			return err
		}
		candidates = append(candidates, fmt.Sprintf("candidate:udpcandidate %d udp %d %s %d typ host", id, basePriority, c, port.ListeningAddr.Port))
		basePriority = basePriority + 1
		r.ports = append(r.ports, port)
	}

	r.LocalDescription = sdp.VP8OnlyDescription(r.iceUsername, r.icePassword, r.tlscfg.Fingerprint(), candidates)

	return nil
}

// AddTrack adds a new track to the RTCPeerConnection
// This function returns a channel to push buffers on, and an error if the channel can't be added
// Closing the channel ends this stream
func (r *RTCPeerConnection) AddTrack(mediaType TrackType) (buffers chan<- []byte, err error) {
	trackInput := make(chan []byte, 15)
	go func() {
		for {
			<-trackInput
			fmt.Println("TODO Discarding packet, need media parsing")

			// rtpPacket := <-trackInput
			// for _, p := range r.ports {
			// 	p.Send(rtpPacket)
			// }
		}
	}()
	return trackInput, nil
}

// Close ends the RTCPeerConnection
func (r *RTCPeerConnection) Close() error {
	r.portsLock.Lock()
	defer r.portsLock.Unlock()

	// Walk all ports remove and close them
	for _, p := range r.ports {
		if err := p.Close(); err != nil {
			return err
		}
	}
	r.ports = nil
	return nil
}

// Private
func (r *RTCPeerConnection) generateChannel(ssrc uint32) (buffers chan<- *rtp.Packet) {
	if r.Ontrack == nil {
		return nil
	}

	bufferTransport := make(chan *rtp.Packet, 15)
	go r.Ontrack(VP8, bufferTransport) // TODO look up media via SSRC in remote SD
	return bufferTransport
}

// Private
func (r *RTCPeerConnection) iceStateChange(p *network.Port) {
	updateAndNotify := func(newState ice.ConnectionState) {
		if r.OnICEConnectionStateChange != nil && r.iceState != newState {
			r.OnICEConnectionStateChange(newState)
		}
		r.iceState = newState
	}

	if p.ICEState == ice.Failed {
		if err := p.Close(); err != nil {
			fmt.Println(errors.Wrap(err, "Failed to close Port when ICE went to failed"))
		}

		r.portsLock.Lock()
		defer r.portsLock.Unlock()
		for i := len(r.ports) - 1; i >= 0; i-- {
			if r.ports[i] == p {
				r.ports = append(r.ports[:i], r.ports[i+1:]...)
			}
		}

		if len(r.ports) == 0 {
			updateAndNotify(ice.Disconnected)
		}
	} else {
		updateAndNotify(ice.Connected)
	}
}
