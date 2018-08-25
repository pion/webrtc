// Package webrtc implements the WebRTC 1.0 as defined in W3C WebRTC specification document.
package webrtc

import (
	"fmt"
	"github.com/pions/webrtc/internal/network"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
	"sync"
	"time"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
)

func init() {
	// TODO Must address this and either revert or delete.
	// rand.Seed(time.Now().UTC().UnixNano())
}

// RTCPeerConnection represents a WebRTC connection between itself and a remote peer
type RTCPeerConnection struct {
	sync.RWMutex

	configuration RTCConfiguration

	// ICE
	OnICEConnectionStateChange func(iceConnectionState ice.ConnectionState)
	IceConnectionState         ice.ConnectionState

	networkManager *network.Manager

	// Signaling
	CurrentLocalDescription *RTCSessionDescription
	PendingLocalDescription *RTCSessionDescription

	CurrentRemoteDescription *RTCSessionDescription
	PendingRemoteDescription *RTCSessionDescription

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

	// SCTP
	sctp *RTCSctpTransport

	// DataChannels
	dataChannels  map[uint16]*RTCDataChannel
	Ondatachannel func(*RTCDataChannel)
}

// Public

// New creates a new RTCPeerConfiguration with the provided configuration
func New(configuration RTCConfiguration) (*RTCPeerConnection, error) {
	// Some variables defined explicitly despite their implicit zero values to
	// allow better readability to understand what is happening.
	pc := RTCPeerConnection{
		configuration: RTCConfiguration{
			IceServers:           []RTCIceServer{},
			IceTransportPolicy:   RTCIceTransportPolicyAll,
			BundlePolicy:         RTCBundlePolicyBalanced,
			RtcpMuxPolicy:        RTCRtcpMuxPolicyRequire,
			Certificates:         []RTCCertificate{},
			IceCandidatePoolSize: 0,
		},
		signalingState:  RTCSignalingStateStable,
		connectionState: RTCPeerConnectionStateNew,
		mediaEngine:     DefaultMediaEngine,
		sctp:            newRTCSctpTransport(),
		dataChannels:    make(map[uint16]*RTCDataChannel),
	}
	var err error

	if err = pc.initConfiguration(configuration); err != nil {
		return nil, err
	}

	pc.networkManager, err = network.NewManager(pc.generateChannel, pc.dataChannelEventHandler, pc.iceStateChange)
	if err != nil {
		return nil, err
	}

	// https://www.w3.org/TR/webrtc/#constructor (step #4)
	// This validation is omitted since the pions-webrtc implements rtcp-mux.
	// FIXME This is actually not implemented yet but will be soon.

	return &pc, nil
}

// initConfiguration defines validation of the specified RTCConfiguration and
// its assignment to the internal configuration variable. This function differs
// from its SetConfiguration counterpart because most of the checks do not
// include verification statements related to the existing state. Thus the
// function describes only minor verification of some the struct variables.
func (pc *RTCPeerConnection) initConfiguration(configuration RTCConfiguration) error {
	if configuration.PeerIdentity != "" {
		pc.configuration.PeerIdentity = configuration.PeerIdentity
	}

	// https://www.w3.org/TR/webrtc/#constructor (step #3)
	if len(configuration.Certificates) > 0 {
		now := time.Now()
		for _, x509Cert := range configuration.Certificates {
			if !x509Cert.Expires().IsZero() && now.After(x509Cert.Expires()) {
				return &InvalidAccessError{Err: ErrCertificateExpired}
			}
		}
	} else {
		sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return &UnknownError{Err: err}
		}
		certificate, err := GenerateCertificate(sk)
		if err != nil {
			return err
		}
		pc.configuration.Certificates = []RTCCertificate{*certificate}
	}

	if configuration.BundlePolicy != 0 {
		pc.configuration.BundlePolicy = configuration.BundlePolicy
	}

	if configuration.RtcpMuxPolicy != 0 {
		pc.configuration.RtcpMuxPolicy = configuration.RtcpMuxPolicy
	}

	if configuration.IceCandidatePoolSize != 0 {
		pc.configuration.IceCandidatePoolSize = configuration.IceCandidatePoolSize
	}

	if configuration.IceTransportPolicy != 0 {
		pc.configuration.IceTransportPolicy = configuration.IceTransportPolicy
	}

	if len(configuration.IceServers) > 0 {
		for _, server := range configuration.IceServers {
			if err := server.validate(); err != nil {
				return err
			}
		}

		pc.configuration.IceServers = configuration.IceServers
	}

	return nil
}

// SetConfiguration updates the configuration of this RTCPeerConnection object.
func (pc *RTCPeerConnection) SetConfiguration(configuration RTCConfiguration) error {
	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-setconfiguration (step #2)
	if pc.IsClosed {
		return &InvalidStateError{Err: ErrConnectionClosed}
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #3)
	if configuration.PeerIdentity != "" {
		if configuration.PeerIdentity != pc.configuration.PeerIdentity {
			return &InvalidModificationError{Err: ErrModifyingPeerIdentity}
		}
		pc.configuration.PeerIdentity = configuration.PeerIdentity
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #4)
	if len(configuration.Certificates) > 0 {
		if len(configuration.Certificates) != len(pc.configuration.Certificates) {
			return &InvalidModificationError{Err: ErrModifyingCertificates}
		}

		for i, certificate := range configuration.Certificates {
			if !pc.configuration.Certificates[i].Equals(certificate) {
				return &InvalidModificationError{Err: ErrModifyingCertificates}
			}
		}
		pc.configuration.Certificates = configuration.Certificates
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #5)
	if configuration.BundlePolicy != 0 {
		if configuration.BundlePolicy != pc.configuration.BundlePolicy {
			return &InvalidModificationError{Err: ErrModifyingBundlePolicy}
		}
		pc.configuration.BundlePolicy = configuration.BundlePolicy
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #6)
	if configuration.RtcpMuxPolicy != 0 {
		if configuration.RtcpMuxPolicy != pc.configuration.RtcpMuxPolicy {
			return &InvalidModificationError{Err: ErrModifyingRtcpMuxPolicy}
		}
		pc.configuration.RtcpMuxPolicy = configuration.RtcpMuxPolicy
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #7)
	if configuration.IceCandidatePoolSize != 0 {
		if pc.configuration.IceCandidatePoolSize != configuration.IceCandidatePoolSize &&
			pc.LocalDescription() != nil {
			return &InvalidModificationError{Err: ErrModifyingIceCandidatePoolSize}
		}
		pc.configuration.IceCandidatePoolSize = configuration.IceCandidatePoolSize
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #8)
	if configuration.IceTransportPolicy != 0 {
		pc.configuration.IceTransportPolicy = configuration.IceTransportPolicy
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #11)
	if len(configuration.IceServers) > 0 {
		// https://www.w3.org/TR/webrtc/#set-the-configuration (step #11.3)
		for _, server := range configuration.IceServers {
			if err := server.validate(); err != nil {
				return err
			}
		}
		pc.configuration.IceServers = configuration.IceServers
	}

	return nil
}

// GetConfiguration returns an RTCConfiguration object representing the current
// configuration of this RTCPeerConnection object. The returned object is a
// copy and direct mutation on it will not take affect until SetConfiguration
// has been called with RTCConfiguration passed as its only arguement.
// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-getconfiguration
func (pc *RTCPeerConnection) GetConfiguration() RTCConfiguration {
	return pc.configuration
}

// LocalDescription returns PendingLocalDescription if it is not null and
// otherwise it returns CurrentLocalDescription. This property is used to
// determine if setLocalDescription has already been called.
// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-localdescription
func (pc *RTCPeerConnection) LocalDescription() *RTCSessionDescription {
	if pc.PendingLocalDescription != nil {
		return pc.PendingLocalDescription
	}
	return pc.CurrentLocalDescription
}

// RemoteDescription returns PendingRemoteDescription if it is not null and
// otherwise it returns CurrentRemoteDescription. This property is used to
// determine if setRemoteDescription has already been called.
// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-remotedescription
func (pc *RTCPeerConnection) RemoteDescription() *RTCSessionDescription {
	if pc.PendingRemoteDescription != nil {
		return pc.PendingRemoteDescription
	}
	return pc.CurrentRemoteDescription
}

// SetMediaEngine allows overwriting the default media engine used by the RTCPeerConnection
// This enables RTCPeerConnection with support for different codecs
func (pc *RTCPeerConnection) SetMediaEngine(m *MediaEngine) {
	pc.mediaEngine = m
}

// SetIdentityProvider is used to configure an identity provider to generate identity assertions
func (pc *RTCPeerConnection) SetIdentityProvider(provider string) error {
	return errors.Errorf("TODO SetIdentityProvider")
}

// Close ends the RTCPeerConnection
func (pc *RTCPeerConnection) Close() error {
	pc.networkManager.Close()

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #2)
	if pc.IsClosed {
		return nil
	}

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #3)
	pc.IsClosed = true

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #4)
	pc.signalingState = RTCSignalingStateClosed

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #11)
	pc.IceConnectionState = ice.ConnectionStateClosed

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #12)
	pc.connectionState = RTCPeerConnectionStateClosed

	return nil
}

/* Everything below is private */
func (pc *RTCPeerConnection) generateChannel(ssrc uint32, payloadType uint8) (buffers chan<- *rtp.Packet) {
	if pc.Ontrack == nil {
		return nil
	}

	sdpCodec, err := pc.CurrentLocalDescription.parsed.GetCodecForPayloadType(payloadType)
	if err != nil {
		fmt.Printf("No codec could be found in RemoteDescription for payloadType %d \n", payloadType)
		return nil
	}

	codec, err := pc.mediaEngine.getCodecSDP(sdpCodec)
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

	go pc.Ontrack(track)
	return bufferTransport
}

func (pc *RTCPeerConnection) iceStateChange(newState ice.ConnectionState) {
	pc.Lock()
	defer pc.Unlock()

	if pc.OnICEConnectionStateChange != nil && pc.IceConnectionState != newState {
		pc.OnICEConnectionStateChange(newState)
	}
	pc.IceConnectionState = newState
}

func (pc *RTCPeerConnection) dataChannelEventHandler(e network.DataChannelEvent) {
	pc.Lock()
	defer pc.Unlock()

	switch event := e.(type) {
	case *network.DataChannelCreated:
		newDataChannel := &RTCDataChannel{ID: event.StreamIdentifier(), Label: event.Label, rtcPeerConnection: pc}
		pc.dataChannels[e.StreamIdentifier()] = newDataChannel
		if pc.Ondatachannel != nil {
			go pc.Ondatachannel(newDataChannel)
		} else {
			fmt.Println("Ondatachannel is unset, discarding message")
		}
	case *network.DataChannelMessage:
		if datachannel, ok := pc.dataChannels[e.StreamIdentifier()]; ok {
			datachannel.RLock()
			defer datachannel.RUnlock()

			if datachannel.Onmessage != nil {
				go datachannel.Onmessage(event.Payload)
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
