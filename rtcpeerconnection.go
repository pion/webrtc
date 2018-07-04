package webrtc

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/network"
	"github.com/pions/webrtc/internal/sdp"
	"github.com/pions/webrtc/internal/util"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pions/webrtc/pkg/rtp/codecs"

	"github.com/pkg/errors"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// RTCSample contains media, and the amount of samples in it
type RTCSample struct {
	Data    []byte
	Samples uint32
}

// TrackType determines the type of media we are sending receiving
type TrackType int

// List of supported TrackTypes
const (
	VP8 TrackType = iota + 1
	VP9
	Opus
)

func (t TrackType) String() string {
	switch t {
	case VP8:
		return "VP8"
	case VP9:
		return "VP9"
	case Opus:
		return "Opus"
	default:
		return "Unknown"
	}
}

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

	remoteDescription *sdp.SessionDescription

	localTracks []*sdp.SessionBuilderTrack
}

// Public

// SetRemoteDescription sets the SessionDescription of the remote peer
func (r *RTCPeerConnection) SetRemoteDescription(rawSessionDescription string) error {
	if r.remoteDescription != nil {
		return errors.Errorf("remoteDescription is already defined, SetRemoteDescription can only be called once")
	}

	r.remoteDescription = &sdp.SessionDescription{}
	return r.remoteDescription.Unmarshal(rawSessionDescription)
}

// CreateOffer starts the RTCPeerConnection and generates the localDescription
func (r *RTCPeerConnection) CreateOffer() error {
	return errors.Errorf("CreateOffer is not implemented")
}

// CreateAnswer starts the RTCPeerConnection and generates the localDescription
func (r *RTCPeerConnection) CreateAnswer() error {
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
		candidates = append(candidates, fmt.Sprintf("candidate:udpcandidate %d udp %d %s %d typ host", id+1, basePriority, c, port.ListeningAddr.Port))
		basePriority = basePriority + 1
		r.ports = append(r.ports, port)
	}

	r.LocalDescription = sdp.BaseSessionDescription(&sdp.SessionBuilder{
		IceUsername: r.iceUsername,
		IcePassword: r.icePassword,
		Fingerprint: r.tlscfg.Fingerprint(),
		Candidates:  candidates,
		Tracks:      r.localTracks,
	})

	return nil
}

// AddTrack adds a new track to the RTCPeerConnection
// This function returns a channel to push buffers on, and an error if the channel can't be added
// Closing the channel ends this stream
func (r *RTCPeerConnection) AddTrack(mediaType TrackType, clockRate uint32) (samples chan<- RTCSample, err error) {
	if mediaType != VP8 && mediaType != Opus {
		panic("TODO Discarding packet, need media parsing")
	}

	trackInput := make(chan RTCSample, 15)
	go func() {
		ssrc := rand.Uint32()
		sdpTrack := &sdp.SessionBuilderTrack{SSRC: ssrc}
		var payloader rtp.Payloader
		var payloadType uint8
		switch mediaType {
		case Opus:
			sdpTrack.IsAudio = true
			payloader = &codecs.OpusPayloader{}
			payloadType = 111

		case VP8:
			payloader = &codecs.VP8Payloader{}
			payloadType = 96
		}

		r.localTracks = append(r.localTracks, sdpTrack)
		packetizer := rtp.NewPacketizer(1500, payloadType, ssrc, payloader, rtp.NewRandomSequencer(), clockRate)
		for {
			in := <-trackInput
			packets := packetizer.Packetize(in.Data, in.Samples)
			for _, p := range packets {
				for _, port := range r.ports {
					port.Send(p)
				}
			}
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
func (r *RTCPeerConnection) generateChannel(ssrc uint32, payloadType uint8) (buffers chan<- *rtp.Packet) {
	if r.Ontrack == nil {
		return nil
	}

	var codec TrackType
	ok, codecStr := sdp.GetCodecForPayloadType(payloadType, r.remoteDescription)
	if !ok {
		fmt.Printf("No codec could be found in RemoteDescription for payloadType %d \n", payloadType)
		return nil
	}

	switch codecStr {
	case "VP8":
		codec = VP8
	case "VP9":
		codec = VP9
	case "opus":
		codec = Opus
	default:
		fmt.Printf("Codec %s in not supported by pion-WebRTC \n", codecStr)
		return nil
	}

	bufferTransport := make(chan *rtp.Packet, 15)
	go r.Ontrack(codec, bufferTransport)
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
