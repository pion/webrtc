package webrtc

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/pions/webrtc/internal/network"
	"github.com/pions/webrtc/internal/sdp"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtp"
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

	// ICE: TODO: Move to ICEAgent
	iceAgent           *ice.Agent
	iceState           ice.ConnectionState
	iceGatheringState  ice.GatheringState
	iceConnectionState ice.ConnectionState

	networkManager *network.Manager

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
	mediaEngine     *MediaEngine
	rtpTransceivers []*RTCRtpTransceiver
	Ontrack         func(*RTCTrack)

	// DataChannels
	dataChannels  map[uint16]*RTCDataChannel
	Ondatachannel func(*RTCDataChannel)
}

// Public

// New creates a new RTCPeerConfiguration with the provided configuration
func New(config RTCConfiguration) (*RTCPeerConnection, error) {
	r := &RTCPeerConnection{
		config:             config,
		signalingState:     RTCSignalingStateStable,
		iceAgent:           ice.NewAgent(),
		iceGatheringState:  ice.GatheringStateNew,
		iceConnectionState: ice.ConnectionStateNew,
		connectionState:    RTCPeerConnectionStateNew,
		mediaEngine:        DefaultMediaEngine,
		dataChannels:       make(map[uint16]*RTCDataChannel),
	}
	err := r.SetConfiguration(config)
	if err != nil {
		return nil, err
	}

	r.networkManager, err = network.NewManager([]byte(r.iceAgent.Pwd), r.generateChannel, r.dataChannelEventHandler, r.iceStateChange)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// SetMediaEngine allows overwriting the default media engine used by the RTCPeerConnection
// This enables RTCPeerConnection with support for different codecs
func (r *RTCPeerConnection) SetMediaEngine(m *MediaEngine) {
	r.mediaEngine = m
}

// SetIdentityProvider is used to configure an identity provider to generate identity assertions
func (r *RTCPeerConnection) SetIdentityProvider(provider string) error {
	panic("TODO SetIdentityProvider")
}

// Close ends the RTCPeerConnection
func (r *RTCPeerConnection) Close() error {
	r.networkManager.Close()
	return nil
}

/* Everything below is private */
func (r *RTCPeerConnection) generateChannel(ssrc uint32, payloadType uint8) (buffers chan<- *rtp.Packet) {
	if r.Ontrack == nil {
		return nil
	}

	sdpCodec, err := r.remoteDescription.GetCodecForPayloadType(payloadType)
	if err != nil {
		fmt.Printf("No codec could be found in RemoteDescription for payloadType %d \n", payloadType)
		return nil
	}

	codec, err := r.mediaEngine.getCodecSDP(sdpCodec)
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

func (r *RTCPeerConnection) iceStateChange(newState ice.ConnectionState) {
	if r.OnICEConnectionStateChange != nil && r.iceState != newState {
		r.OnICEConnectionStateChange(newState)
	}
	r.iceState = newState
}

func (r *RTCPeerConnection) dataChannelEventHandler(e network.DataChannelEvent) {
	switch event := e.(type) {
	case *network.DataChannelCreated:
		newDataChannel := &RTCDataChannel{ID: event.StreamIdentifier(), Label: event.Label}
		r.dataChannels[e.StreamIdentifier()] = newDataChannel
		if r.Ondatachannel != nil {
			go r.Ondatachannel(newDataChannel)
		} else {
			fmt.Println("Ondatachannel is unset, discarding message")
		}
	case *network.DataChannelMessage:
		if datachannel, ok := r.dataChannels[e.StreamIdentifier()]; ok {
			if datachannel.Onmessage != nil {
				go datachannel.Onmessage(event.Body)
			} else {
				fmt.Printf("Onmessage has not been set for Datachannel %s %d \n", datachannel.Label, e.StreamIdentifier())
			}
		} else {
			fmt.Printf("No datachannel found for streamIdentifier %d \n", e.StreamIdentifier())

		}
	default:
		fmt.Printf("Unhandled DataChannelEvent %v \n", event)
	}
}
