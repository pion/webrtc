package webrtc

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/network"
	"github.com/pions/webrtc/internal/sdp"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtp"

	"github.com/pkg/errors"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// RTCPeerConnectionState indicates the state of the peer connection
type RTCPeerConnectionState int

const (

	// RTCPeerConnectionStateNew indicates some of the ICE or DTLS transports are in status new
	RTCPeerConnectionStateNew RTCPeerConnectionState = iota + 1

	// RTCPeerConnectionStateConnecting indicates some of the ICE or DTLS transports are in status connecting or checking
	RTCPeerConnectionStateConnecting

	// RTCPeerConnectionStateConnected indicates all of the ICE or DTLS transports are in status connected or completed
	RTCPeerConnectionStateConnected

	// RTCPeerConnectionStateDisconnected indicates some of the ICE or DTLS transports are in status disconnected
	RTCPeerConnectionStateDisconnected

	// RTCPeerConnectionStateFailed indicates some of the ICE or DTLS transports are in status failed
	RTCPeerConnectionStateFailed

	// RTCPeerConnectionStateClosed indicates the peer connection is closed
	RTCPeerConnectionStateClosed
)

func (t RTCPeerConnectionState) String() string {
	switch t {
	case RTCPeerConnectionStateNew:
		return "new"
	case RTCPeerConnectionStateConnecting:
		return "connecting"
	case RTCPeerConnectionStateConnected:
		return "connected"
	case RTCPeerConnectionStateDisconnected:
		return "disconnected"
	case RTCPeerConnectionStateFailed:
		return "failed"
	case RTCPeerConnectionStateClosed:
		return "closed"
	default:
		return "Unknown"
	}
}

// RTCPeerConnection represents a WebRTC connection between itself and a remote peer
type RTCPeerConnection struct {
	// ICE
	OnICEConnectionStateChange func(iceConnectionState ice.ConnectionState)

	config RTCConfiguration
	tlscfg *dtls.TLSCfg

	// ICE: TODO: Move to ICEAgent
	iceAgent           *ice.Agent
	iceState           ice.ConnectionState
	iceGatheringState  ice.GatheringState
	iceConnectionState ice.ConnectionState

	portsLock sync.RWMutex
	ports     []*network.Port

	// Signaling
	// pendingLocalDescription *RTCSessionDescription
	// currentLocalDescription *RTCSessionDescription
	LocalDescription *sdp.SessionDescription

	// pendingRemoteDescription *RTCSessionDescription
	currentRemoteDescription *RTCSessionDescription
	remoteDescription        *sdp.SessionDescription

	idpLoginURL *string

	IsClosed          bool
	NegotiationNeeded bool

	// lastOffer  string
	// lastAnswer string

	signalingState  RTCSignalingState
	connectionState RTCPeerConnectionState

	// Media
	rtpTransceivers []*RTCRtpTransceiver
	Ontrack         func(*RTCTrack)
}

// New creates a new RTCPeerConfiguration with the provided configuration
func New(config RTCConfiguration) (*RTCPeerConnection, error) {

	r := &RTCPeerConnection{
		config:             config,
		signalingState:     RTCSignalingStateStable,
		iceAgent:           ice.NewAgent(),
		iceGatheringState:  ice.GatheringStateNew,
		iceConnectionState: ice.ConnectionStateNew,
		connectionState:    RTCPeerConnectionStateNew,
	}
	err := r.SetConfiguration(config)
	if err != nil {
		return nil, err
	}

	r.tlscfg = dtls.NewTLSCfg()

	// TODO: Initialize ICE Agent

	return r, nil
}

// Public

// SetIdentityProvider is used to configure an identity provider to generate identity assertions
func (r *RTCPeerConnection) SetIdentityProvider(provider string) error {
	panic("TODO SetIdentityProvider")
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

	sdpCodec, err := r.remoteDescription.GetCodecForPayloadType(payloadType)
	if err != nil {
		fmt.Printf("No codec could be found in RemoteDescription for payloadType %d \n", payloadType)
		return nil
	}

	codec, err := rtcMediaEngine.getCodecSDP(sdpCodec)
	if err != nil {
		fmt.Printf("Codec %s in not registered\n", sdpCodec)
	}

	bufferTransport := make(chan *rtp.Packet, 15)

	track := &RTCTrack{
		PayloadType: payloadType,
		Kind:        codec.Type,
		ID:          "0", // TODO extract from remoteDescription
		Label:       "",  // TODO extract from remoteDescription
		Ssrc:        ssrc,
		Codec:       codec,
		Packets:     bufferTransport,
	}

	// TODO: Register the receiving Track

	go r.Ontrack(track)
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

	if p.ICEState == ice.ConnectionStateFailed {
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
			updateAndNotify(ice.ConnectionStateDisconnected)
		}
	} else {
		updateAndNotify(ice.ConnectionStateConnected)
	}
}
