// +build !js

// Package webrtc implements the WebRTC 1.0 as defined in W3C WebRTC specification document.
package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pions/ice"
	"github.com/pions/logging"
	"github.com/pions/rtcp"
	"github.com/pions/sdp/v2"
	"github.com/pions/webrtc/internal/util"
	"github.com/pions/webrtc/pkg/rtcerr"
)

// PeerConnection represents a WebRTC connection that establishes a
// peer-to-peer communications with another PeerConnection instance in a
// browser, or to another endpoint implementing the required protocols.
type PeerConnection struct {
	mu sync.RWMutex

	configuration Configuration

	currentLocalDescription  *SessionDescription
	pendingLocalDescription  *SessionDescription
	currentRemoteDescription *SessionDescription
	pendingRemoteDescription *SessionDescription
	signalingState           SignalingState
	iceGatheringState        ICEGatheringState
	iceConnectionState       ICEConnectionState
	connectionState          PeerConnectionState

	idpLoginURL *string

	isClosed          bool
	negotiationNeeded bool

	lastOffer  string
	lastAnswer string

	rtpTransceivers []*RTPTransceiver

	// DataChannels
	dataChannels map[uint16]*DataChannel

	// OnNegotiationNeeded        func() // FIXME NOT-USED
	// OnICECandidateError        func() // FIXME NOT-USED

	// OnConnectionStateChange    func() // FIXME NOT-USED

	onSignalingStateChangeHandler     func(SignalingState)
	onICEConnectionStateChangeHandler func(ICEConnectionState)
	onTrackHandler                    func(*Track, *RTPReceiver)
	onDataChannelHandler              func(*DataChannel)
	onICECandidateHandler             func(*ICECandidate)
	onICEGatheringStateChangeHandler  func()

	iceGatherer   *ICEGatherer
	iceTransport  *ICETransport
	dtlsTransport *DTLSTransport
	sctpTransport *SCTPTransport

	// A reference to the associated API state used by this connection
	api *API
	log *logging.LeveledLogger
}

// NewPeerConnection creates a peerconnection with the default
// codecs. See API.NewRTCPeerConnection for details.
func NewPeerConnection(configuration Configuration) (*PeerConnection, error) {
	m := MediaEngine{}
	m.RegisterDefaultCodecs()
	api := NewAPI(WithMediaEngine(m))
	return api.NewPeerConnection(configuration)
}

// NewPeerConnection creates a new PeerConnection with the provided configuration against the received API object
func (api *API) NewPeerConnection(configuration Configuration) (*PeerConnection, error) {
	// https://w3c.github.io/webrtc-pc/#constructor (Step #2)
	// Some variables defined explicitly despite their implicit zero values to
	// allow better readability to understand what is happening.
	pc := &PeerConnection{
		configuration: Configuration{
			ICEServers:           []ICEServer{},
			ICETransportPolicy:   ICETransportPolicyAll,
			BundlePolicy:         BundlePolicyBalanced,
			RTCPMuxPolicy:        RTCPMuxPolicyRequire,
			Certificates:         []Certificate{},
			ICECandidatePoolSize: 0,
		},
		isClosed:           false,
		negotiationNeeded:  false,
		lastOffer:          "",
		lastAnswer:         "",
		signalingState:     SignalingStateStable,
		iceConnectionState: ICEConnectionStateNew,
		iceGatheringState:  ICEGatheringStateNew,
		connectionState:    PeerConnectionStateNew,
		dataChannels:       make(map[uint16]*DataChannel),

		api: api,
		log: logging.NewScopedLogger("pc"),
	}

	var err error
	if err = pc.initConfiguration(configuration); err != nil {
		return nil, err
	}

	// For now we eagerly allocate and start the gatherer
	gatherer, err := pc.createICEGatherer()
	if err != nil {
		return nil, err
	}
	pc.iceGatherer = gatherer

	err = pc.gather()

	if err != nil {
		return nil, err
	}

	// Create the ice transport
	iceTransport := pc.createICETransport()
	pc.iceTransport = iceTransport

	// Create the DTLS transport
	dtlsTransport, err := pc.createDTLSTransport()
	if err != nil {
		return nil, err
	}
	pc.dtlsTransport = dtlsTransport

	return pc, nil
}

// initConfiguration defines validation of the specified Configuration and
// its assignment to the internal configuration variable. This function differs
// from its SetConfiguration counterpart because most of the checks do not
// include verification statements related to the existing state. Thus the
// function describes only minor verification of some the struct variables.
func (pc *PeerConnection) initConfiguration(configuration Configuration) error {
	if configuration.PeerIdentity != "" {
		pc.configuration.PeerIdentity = configuration.PeerIdentity
	}

	// https://www.w3.org/TR/webrtc/#constructor (step #3)
	if len(configuration.Certificates) > 0 {
		now := time.Now()
		for _, x509Cert := range configuration.Certificates {
			if !x509Cert.Expires().IsZero() && now.After(x509Cert.Expires()) {
				return &rtcerr.InvalidAccessError{Err: ErrCertificateExpired}
			}
			pc.configuration.Certificates = append(pc.configuration.Certificates, x509Cert)
		}
	} else {
		sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return &rtcerr.UnknownError{Err: err}
		}
		certificate, err := GenerateCertificate(sk)
		if err != nil {
			return err
		}
		pc.configuration.Certificates = []Certificate{*certificate}
	}

	if configuration.BundlePolicy != BundlePolicy(Unknown) {
		pc.configuration.BundlePolicy = configuration.BundlePolicy
	}

	if configuration.RTCPMuxPolicy != RTCPMuxPolicy(Unknown) {
		pc.configuration.RTCPMuxPolicy = configuration.RTCPMuxPolicy
	}

	if configuration.ICECandidatePoolSize != 0 {
		pc.configuration.ICECandidatePoolSize = configuration.ICECandidatePoolSize
	}

	if configuration.ICETransportPolicy != ICETransportPolicy(Unknown) {
		pc.configuration.ICETransportPolicy = configuration.ICETransportPolicy
	}

	if len(configuration.ICEServers) > 0 {
		for _, server := range configuration.ICEServers {
			if _, err := server.validate(); err != nil {
				return err
			}
		}
		pc.configuration.ICEServers = configuration.ICEServers
	}

	return nil
}

// OnSignalingStateChange sets an event handler which is invoked when the
// peer connection's signaling state changes
func (pc *PeerConnection) OnSignalingStateChange(f func(SignalingState)) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.onSignalingStateChangeHandler = f
}

func (pc *PeerConnection) onSignalingStateChange(newState SignalingState) (done chan struct{}) {
	pc.mu.RLock()
	hdlr := pc.onSignalingStateChangeHandler
	pc.mu.RUnlock()

	pc.log.Infof("signaling state changed to %s", newState)
	done = make(chan struct{})
	if hdlr == nil {
		close(done)
		return
	}

	go func() {
		hdlr(newState)
		close(done)
	}()

	return
}

// OnDataChannel sets an event handler which is invoked when a data
// channel message arrives from a remote peer.
func (pc *PeerConnection) OnDataChannel(f func(*DataChannel)) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.onDataChannelHandler = f
}

// OnICECandidate sets an event handler which is invoked when a new ICE
// candidate is found.
// BUG: trickle ICE is not supported so this event is triggered immediately when
// SetLocalDescription is called. Typically, you only need to use this method
// if you want API compatibility with the JavaScript/Wasm bindings.
func (pc *PeerConnection) OnICECandidate(f func(*ICECandidate)) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.onICECandidateHandler = f
}

// OnICEGatheringStateChange sets an event handler which is invoked when the
// ICE candidate gathering state has changed.
// BUG: trickle ICE is not supported so this event is triggered immediately when
// SetLocalDescription is called. Typically, you only need to use this method
// if you want API compatibility with the JavaScript/Wasm bindings.
func (pc *PeerConnection) OnICEGatheringStateChange(f func()) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.onICEGatheringStateChangeHandler = f
}

// signalICECandidateGatheringComplete should be called after ICE candidate
// gathering is complete. It triggers the appropriate event handlers in order to
// emulate a trickle ICE process.
func (pc *PeerConnection) signalICECandidateGatheringComplete() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Call onICECandidateHandler for all candidates.
	if pc.onICECandidateHandler != nil {
		candidates, err := pc.iceGatherer.GetLocalCandidates()
		if err != nil {
			return err
		}
		for i := range candidates {
			go pc.onICECandidateHandler(&candidates[i])
		}
		// Call the handler one last time with nil. This is a signal that candidate
		// gathering is complete.
		go pc.onICECandidateHandler(nil)
	}

	pc.iceGatheringState = ICEGatheringStateComplete

	// Also trigger the onICEGatheringStateChangeHandler
	if pc.onICEGatheringStateChangeHandler != nil {
		// Note: Gathering is already done at this point, but some clients might
		// still expect the state change handler to be triggered.
		go pc.onICEGatheringStateChangeHandler()
	}

	return nil
}

// OnTrack sets an event handler which is called when remote track
// arrives from a remote peer.
func (pc *PeerConnection) OnTrack(f func(*Track, *RTPReceiver)) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.onTrackHandler = f
}

func (pc *PeerConnection) onTrack(t *Track, r *RTPReceiver) (done chan struct{}) {
	pc.mu.RLock()
	hdlr := pc.onTrackHandler
	pc.mu.RUnlock()

	pc.log.Debugf("got new track: %+v", t)
	done = make(chan struct{})
	if hdlr == nil || t == nil {
		close(done)
		return
	}

	go func() {
		hdlr(t, r)
		close(done)
	}()

	return
}

// OnICEConnectionStateChange sets an event handler which is called
// when an ICE connection state is changed.
func (pc *PeerConnection) OnICEConnectionStateChange(f func(ICEConnectionState)) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.onICEConnectionStateChangeHandler = f
}

func (pc *PeerConnection) onICEConnectionStateChange(cs ICEConnectionState) (done chan struct{}) {
	pc.mu.RLock()
	hdlr := pc.onICEConnectionStateChangeHandler
	pc.mu.RUnlock()

	pc.log.Infof("ICE connection state changed: %s", cs)
	done = make(chan struct{})
	if hdlr == nil {
		close(done)
		return
	}

	go func() {
		hdlr(cs)
		close(done)
	}()

	return
}

// SetConfiguration updates the configuration of this PeerConnection object.
func (pc *PeerConnection) SetConfiguration(configuration Configuration) error {
	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-setconfiguration (step #2)
	if pc.isClosed {
		return &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #3)
	if configuration.PeerIdentity != "" {
		if configuration.PeerIdentity != pc.configuration.PeerIdentity {
			return &rtcerr.InvalidModificationError{Err: ErrModifyingPeerIdentity}
		}
		pc.configuration.PeerIdentity = configuration.PeerIdentity
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #4)
	if len(configuration.Certificates) > 0 {
		if len(configuration.Certificates) != len(pc.configuration.Certificates) {
			return &rtcerr.InvalidModificationError{Err: ErrModifyingCertificates}
		}

		for i, certificate := range configuration.Certificates {
			if !pc.configuration.Certificates[i].Equals(certificate) {
				return &rtcerr.InvalidModificationError{Err: ErrModifyingCertificates}
			}
		}
		pc.configuration.Certificates = configuration.Certificates
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #5)
	if configuration.BundlePolicy != BundlePolicy(Unknown) {
		if configuration.BundlePolicy != pc.configuration.BundlePolicy {
			return &rtcerr.InvalidModificationError{Err: ErrModifyingBundlePolicy}
		}
		pc.configuration.BundlePolicy = configuration.BundlePolicy
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #6)
	if configuration.RTCPMuxPolicy != RTCPMuxPolicy(Unknown) {
		if configuration.RTCPMuxPolicy != pc.configuration.RTCPMuxPolicy {
			return &rtcerr.InvalidModificationError{Err: ErrModifyingRTCPMuxPolicy}
		}
		pc.configuration.RTCPMuxPolicy = configuration.RTCPMuxPolicy
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #7)
	if configuration.ICECandidatePoolSize != 0 {
		if pc.configuration.ICECandidatePoolSize != configuration.ICECandidatePoolSize &&
			pc.LocalDescription() != nil {
			return &rtcerr.InvalidModificationError{Err: ErrModifyingICECandidatePoolSize}
		}
		pc.configuration.ICECandidatePoolSize = configuration.ICECandidatePoolSize
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #8)
	if configuration.ICETransportPolicy != ICETransportPolicy(Unknown) {
		pc.configuration.ICETransportPolicy = configuration.ICETransportPolicy
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #11)
	if len(configuration.ICEServers) > 0 {
		// https://www.w3.org/TR/webrtc/#set-the-configuration (step #11.3)
		for _, server := range configuration.ICEServers {
			if _, err := server.validate(); err != nil {
				return err
			}
		}
		pc.configuration.ICEServers = configuration.ICEServers
	}
	return nil
}

// GetConfiguration returns a Configuration object representing the current
// configuration of this PeerConnection object. The returned object is a
// copy and direct mutation on it will not take affect until SetConfiguration
// has been called with Configuration passed as its only argument.
// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-getconfiguration
func (pc *PeerConnection) GetConfiguration() Configuration {
	return pc.configuration
}

// ------------------------------------------------------------------------
// --- FIXME - BELOW CODE NEEDS REVIEW/CLEANUP
// ------------------------------------------------------------------------

// CreateOffer starts the PeerConnection and generates the localDescription
func (pc *PeerConnection) CreateOffer(options *OfferOptions) (SessionDescription, error) {
	useIdentity := pc.idpLoginURL != nil
	switch {
	case options != nil:
		return SessionDescription{}, fmt.Errorf("TODO handle options")
	case useIdentity:
		return SessionDescription{}, fmt.Errorf("TODO handle identity provider")
	case pc.isClosed:
		return SessionDescription{}, &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	d := sdp.NewJSEPSessionDescription(useIdentity)
	pc.addFingerprint(d)

	iceParams, err := pc.iceGatherer.GetLocalParameters()
	if err != nil {
		return SessionDescription{}, err
	}

	candidates, err := pc.iceGatherer.GetLocalCandidates()
	if err != nil {
		return SessionDescription{}, err
	}

	bundleValue := "BUNDLE"

	if pc.addRTPMediaSection(d, RTPCodecTypeAudio, "audio", iceParams, RTPTransceiverDirectionSendrecv, candidates, sdp.ConnectionRoleActpass) {
		bundleValue += " audio"
	}
	if pc.addRTPMediaSection(d, RTPCodecTypeVideo, "video", iceParams, RTPTransceiverDirectionSendrecv, candidates, sdp.ConnectionRoleActpass) {
		bundleValue += " video"
	}

	pc.addDataMediaSection(d, "data", iceParams, candidates, sdp.ConnectionRoleActpass)
	d = d.WithValueAttribute(sdp.AttrKeyGroup, bundleValue+" data")

	for _, m := range d.MediaDescriptions {
		m.WithPropertyAttribute("setup:actpass")
	}

	sdp, err := d.Marshal()
	if err != nil {
		return SessionDescription{}, err
	}

	desc := SessionDescription{
		Type:   SDPTypeOffer,
		SDP:    string(sdp),
		parsed: d,
	}
	pc.lastOffer = desc.SDP
	return desc, nil
}

func (pc *PeerConnection) createICEGatherer() (*ICEGatherer, error) {
	g, err := pc.api.NewICEGatherer(ICEGatherOptions{
		ICEServers: pc.configuration.ICEServers,
	})
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (pc *PeerConnection) gather() error {
	return pc.iceGatherer.Gather()
}

func (pc *PeerConnection) createICETransport() *ICETransport {
	t := pc.api.NewICETransport(pc.iceGatherer)

	t.OnConnectionStateChange(func(state ICETransportState) {
		cs := ICEConnectionStateNew
		switch state {
		case ICETransportStateNew:
			cs = ICEConnectionStateNew
		case ICETransportStateChecking:
			cs = ICEConnectionStateChecking
		case ICETransportStateConnected:
			cs = ICEConnectionStateConnected
		case ICETransportStateCompleted:
			cs = ICEConnectionStateCompleted
		case ICETransportStateFailed:
			cs = ICEConnectionStateFailed
		case ICETransportStateDisconnected:
			cs = ICEConnectionStateDisconnected
		case ICETransportStateClosed:
			cs = ICEConnectionStateClosed
		default:
			pc.log.Warnf("OnConnectionStateChange: unhandled ICE state: %s", state)
			return
		}
		pc.iceStateChange(cs)
	})

	return t
}

func (pc *PeerConnection) createDTLSTransport() (*DTLSTransport, error) {
	dtlsTransport, err := pc.api.NewDTLSTransport(pc.iceTransport, pc.configuration.Certificates)
	return dtlsTransport, err
}

// CreateAnswer starts the PeerConnection and generates the localDescription
func (pc *PeerConnection) CreateAnswer(options *AnswerOptions) (SessionDescription, error) {
	useIdentity := pc.idpLoginURL != nil
	switch {
	case options != nil:
		return SessionDescription{}, fmt.Errorf("TODO handle options")
	case useIdentity:
		return SessionDescription{}, fmt.Errorf("TODO handle identity provider")
	case pc.isClosed:
		return SessionDescription{}, &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	iceParams, err := pc.iceGatherer.GetLocalParameters()
	if err != nil {
		return SessionDescription{}, err
	}

	candidates, err := pc.iceGatherer.GetLocalCandidates()
	if err != nil {
		return SessionDescription{}, err
	}

	d := sdp.NewJSEPSessionDescription(useIdentity)
	pc.addFingerprint(d)

	bundleValue := "BUNDLE"
	for _, remoteMedia := range pc.RemoteDescription().parsed.MediaDescriptions {
		// TODO @trivigy better SDP parser
		var peerDirection RTPTransceiverDirection
		midValue := ""
		for _, a := range remoteMedia.Attributes {
			switch {
			case strings.HasPrefix(*a.String(), "mid"):
				midValue = (*a.String())[len("mid:"):]
			case strings.HasPrefix(*a.String(), "sendrecv"):
				peerDirection = RTPTransceiverDirectionSendrecv
			case strings.HasPrefix(*a.String(), "sendonly"):
				peerDirection = RTPTransceiverDirectionSendonly
			case strings.HasPrefix(*a.String(), "recvonly"):
				peerDirection = RTPTransceiverDirectionRecvonly
			}
		}

		appendBundle := func() {
			bundleValue += " " + midValue
		}

		switch {
		case strings.HasPrefix(*remoteMedia.MediaName.String(), "audio"):
			if pc.addRTPMediaSection(d, RTPCodecTypeAudio, midValue, iceParams, peerDirection, candidates, sdp.ConnectionRoleActive) {
				appendBundle()
			}
		case strings.HasPrefix(*remoteMedia.MediaName.String(), "video"):
			if pc.addRTPMediaSection(d, RTPCodecTypeVideo, midValue, iceParams, peerDirection, candidates, sdp.ConnectionRoleActive) {
				appendBundle()
			}
		case strings.HasPrefix(*remoteMedia.MediaName.String(), "application"):
			pc.addDataMediaSection(d, midValue, iceParams, candidates, sdp.ConnectionRoleActive)
			appendBundle()
		}
	}

	d = d.WithValueAttribute(sdp.AttrKeyGroup, bundleValue)

	sdp, err := d.Marshal()
	if err != nil {
		return SessionDescription{}, err
	}

	desc := SessionDescription{
		Type:   SDPTypeAnswer,
		SDP:    string(sdp),
		parsed: d,
	}
	pc.lastAnswer = desc.SDP
	return desc, nil
}

// 4.4.1.6 Set the SessionDescription
func (pc *PeerConnection) setDescription(sd *SessionDescription, op stateChangeOp) error {
	if pc.isClosed {
		return &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	cur := pc.signalingState
	setLocal := stateChangeOpSetLocal
	setRemote := stateChangeOpSetRemote
	newSDPDoesNotMatchOffer := &rtcerr.InvalidModificationError{Err: fmt.Errorf("new sdp does not match previous offer")}
	newSDPDoesNotMatchAnswer := &rtcerr.InvalidModificationError{Err: fmt.Errorf("new sdp does not match previous answer")}

	var nextState SignalingState
	var err error
	switch op {
	case setLocal:
		switch sd.Type {
		// stable->SetLocal(offer)->have-local-offer
		case SDPTypeOffer:
			if sd.SDP != pc.lastOffer {
				return newSDPDoesNotMatchOffer
			}
			nextState, err = checkNextSignalingState(cur, SignalingStateHaveLocalOffer, setLocal, sd.Type)
			if err == nil {
				pc.pendingLocalDescription = sd
			}
		// have-remote-offer->SetLocal(answer)->stable
		// have-local-pranswer->SetLocal(answer)->stable
		case SDPTypeAnswer:
			if sd.SDP != pc.lastAnswer {
				return newSDPDoesNotMatchAnswer
			}
			nextState, err = checkNextSignalingState(cur, SignalingStateStable, setLocal, sd.Type)
			if err == nil {
				pc.currentLocalDescription = sd
				pc.currentRemoteDescription = pc.pendingRemoteDescription
				pc.pendingRemoteDescription = nil
				pc.pendingLocalDescription = nil
			}
		case SDPTypeRollback:
			nextState, err = checkNextSignalingState(cur, SignalingStateStable, setLocal, sd.Type)
			if err == nil {
				pc.pendingLocalDescription = nil
			}
		// have-remote-offer->SetLocal(pranswer)->have-local-pranswer
		case SDPTypePranswer:
			if sd.SDP != pc.lastAnswer {
				return newSDPDoesNotMatchAnswer
			}
			nextState, err = checkNextSignalingState(cur, SignalingStateHaveLocalPranswer, setLocal, sd.Type)
			if err == nil {
				pc.pendingLocalDescription = sd
			}
		default:
			return &rtcerr.OperationError{Err: fmt.Errorf("invalid state change op: %s(%s)", op, sd.Type)}
		}
	case setRemote:
		switch sd.Type {
		// stable->SetRemote(offer)->have-remote-offer
		case SDPTypeOffer:
			nextState, err = checkNextSignalingState(cur, SignalingStateHaveRemoteOffer, setRemote, sd.Type)
			if err == nil {
				pc.pendingRemoteDescription = sd
			}
		// have-local-offer->SetRemote(answer)->stable
		// have-remote-pranswer->SetRemote(answer)->stable
		case SDPTypeAnswer:
			nextState, err = checkNextSignalingState(cur, SignalingStateStable, setRemote, sd.Type)
			if err == nil {
				pc.currentRemoteDescription = sd
				pc.currentLocalDescription = pc.pendingLocalDescription
				pc.pendingRemoteDescription = nil
				pc.pendingLocalDescription = nil
			}
		case SDPTypeRollback:
			nextState, err = checkNextSignalingState(cur, SignalingStateStable, setRemote, sd.Type)
			if err == nil {
				pc.pendingRemoteDescription = nil
			}
		// have-local-offer->SetRemote(pranswer)->have-remote-pranswer
		case SDPTypePranswer:
			nextState, err = checkNextSignalingState(cur, SignalingStateHaveRemotePranswer, setRemote, sd.Type)
			if err == nil {
				pc.pendingRemoteDescription = sd
			}
		default:
			return &rtcerr.OperationError{Err: fmt.Errorf("invalid state change op: %s(%s)", op, sd.Type)}
		}
	default:
		return &rtcerr.OperationError{Err: fmt.Errorf("unhandled state change op: %q", op)}
	}

	if err == nil {
		pc.signalingState = nextState
		pc.onSignalingStateChange(nextState)
	}
	return err
}

// SetLocalDescription sets the SessionDescription of the local peer
func (pc *PeerConnection) SetLocalDescription(desc SessionDescription) error {
	if pc.isClosed {
		return &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	// JSEP 5.4
	if desc.SDP == "" {
		switch desc.Type {
		case SDPTypeAnswer, SDPTypePranswer:
			desc.SDP = pc.lastAnswer
		case SDPTypeOffer:
			desc.SDP = pc.lastOffer
		default:
			return &rtcerr.InvalidModificationError{
				Err: fmt.Errorf("invalid SDP type supplied to SetLocalDescription(): %s", desc.Type),
			}
		}
	}

	// TODO: Initiate ICE candidate gathering?

	desc.parsed = &sdp.SessionDescription{}
	if err := desc.parsed.Unmarshal([]byte(desc.SDP)); err != nil {
		return err
	}
	if err := pc.setDescription(&desc, stateChangeOpSetLocal); err != nil {
		return err
	}

	// Call the appropriate event handlers to signal that ICE candidate gathering
	// is complete. In reality it completed a while ago, but triggering these
	// events helps maintain API compatibility with the JavaScript/Wasm bindings.
	if err := pc.signalICECandidateGatheringComplete(); err != nil {
		return err
	}

	return nil
}

// LocalDescription returns pendingLocalDescription if it is not null and
// otherwise it returns currentLocalDescription. This property is used to
// determine if setLocalDescription has already been called.
// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-localdescription
func (pc *PeerConnection) LocalDescription() *SessionDescription {
	if pc.pendingLocalDescription != nil {
		return pc.pendingLocalDescription
	}
	return pc.currentLocalDescription
}

// SetRemoteDescription sets the SessionDescription of the remote peer
func (pc *PeerConnection) SetRemoteDescription(desc SessionDescription) error {
	// FIXME: Remove this when renegotiation is supported
	if pc.currentRemoteDescription != nil {
		return fmt.Errorf("remoteDescription is already defined, SetRemoteDescription can only be called once")
	}
	if pc.isClosed {
		return &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	desc.parsed = &sdp.SessionDescription{}
	if err := desc.parsed.Unmarshal([]byte(desc.SDP)); err != nil {
		return err
	}
	if err := pc.setDescription(&desc, stateChangeOpSetRemote); err != nil {
		return err
	}

	weOffer := true
	remoteUfrag := ""
	remotePwd := ""
	if desc.Type == SDPTypeOffer {
		weOffer = false
	}

	for _, m := range pc.RemoteDescription().parsed.MediaDescriptions {
		for _, a := range m.Attributes {
			switch {
			case a.IsICECandidate():
				sdpCandidate, err := a.ToICECandidate()
				if err != nil {
					return err
				}

				candidate, err := newICECandidateFromSDP(sdpCandidate)
				if err != nil {
					return err
				}

				if err = pc.iceTransport.AddRemoteCandidate(candidate); err != nil {
					return err
				}
			case strings.HasPrefix(*a.String(), "ice-ufrag"):
				remoteUfrag = (*a.String())[len("ice-ufrag:"):]
			case strings.HasPrefix(*a.String(), "ice-pwd"):
				remotePwd = (*a.String())[len("ice-pwd:"):]
			}
		}
	}

	fingerprint, ok := desc.parsed.Attribute("fingerprint")
	if !ok {
		fingerprint, ok = desc.parsed.MediaDescriptions[0].Attribute("fingerprint")
		if !ok {
			return fmt.Errorf("could not find fingerprint")
		}
	}
	var fingerprintHash string
	parts := strings.Split(fingerprint, " ")
	if len(parts) != 2 {
		return fmt.Errorf("invalid fingerprint")
	}
	fingerprint = parts[1]
	fingerprintHash = parts[0]

	// Create the SCTP transport
	sctp := pc.api.NewSCTPTransport(pc.dtlsTransport)
	pc.sctpTransport = sctp

	// Wire up the on datachannel handler
	sctp.OnDataChannel(func(d *DataChannel) {
		pc.mu.RLock()
		hdlr := pc.onDataChannelHandler
		pc.mu.RUnlock()
		if hdlr != nil {
			hdlr(d)
		}
	})

	go func() {
		// Star the networking in a new routine since it will block until
		// the connection is actually established.

		// Start the ice transport
		iceRole := ICERoleControlled
		if weOffer {
			iceRole = ICERoleControlling
		}
		err := pc.iceTransport.Start(
			pc.iceGatherer,
			ICEParameters{
				UsernameFragment: remoteUfrag,
				Password:         remotePwd,
				ICELite:          false,
			},
			&iceRole,
		)

		if err != nil {
			// TODO: Handle error
			pc.log.Warnf("Failed to start manager: %s", err)
			return
		}

		// Start the dtls transport
		err = pc.dtlsTransport.Start(DTLSParameters{
			Role:         DTLSRoleAuto,
			Fingerprints: []DTLSFingerprint{{Algorithm: fingerprintHash, Value: fingerprint}},
		})
		if err != nil {
			// TODO: Handle error
			pc.log.Warnf("Failed to start manager: %s", err)
			return
		}

		pc.openSRTP()

		for _, tranceiver := range pc.rtpTransceivers {
			if tranceiver.Sender != nil {
				err = tranceiver.Sender.Send(RTPSendParameters{
					Encodings: RTPEncodingParameters{
						RTPCodingParameters{
							SSRC:        tranceiver.Sender.track.SSRC(),
							PayloadType: tranceiver.Sender.track.PayloadType(),
						},
					}})

				if err != nil {
					pc.log.Warnf("Failed to start Sender: %s", err)
				}
			}
		}

		go pc.drainSRTP()

		// Start sctp
		err = pc.sctpTransport.Start(SCTPCapabilities{
			MaxMessageSize: 0,
		})
		if err != nil {
			// TODO: Handle error
			pc.log.Warnf("Failed to start SCTP: %s", err)
			return
		}

		// Open data channels that where created before signaling
		pc.openDataChannels()
	}()

	return nil
}

// openDataChannels opens the existing data channels
func (pc *PeerConnection) openDataChannels() {
	for _, d := range pc.dataChannels {
		err := d.open(pc.sctpTransport)
		if err != nil {
			pc.log.Warnf("failed to open data channel: %s", err)
			continue
		}
	}
}

// openSRTP opens knows inbound SRTP streams from the RemoteDescription
func (pc *PeerConnection) openSRTP() {
	incomingSSRCes := map[uint32]RTPCodecType{}

	for _, media := range pc.RemoteDescription().parsed.MediaDescriptions {
		for _, attr := range media.Attributes {
			var codecType RTPCodecType
			switch media.MediaName.Media {
			case "audio":
				codecType = RTPCodecTypeAudio
			case "video":
				codecType = RTPCodecTypeVideo
			default:
				continue
			}

			if attr.Key == sdp.AttrKeySSRC {
				ssrc, err := strconv.ParseUint(strings.Split(attr.Value, " ")[0], 10, 32)
				if err != nil {
					pc.log.Warnf("Failed to parse SSRC: %v", err)
					continue
				}

				incomingSSRCes[uint32(ssrc)] = codecType
			}
		}
	}

	for i := range incomingSSRCes {
		go func(ssrc uint32, codecType RTPCodecType) {
			receiver, err := pc.api.NewRTPReceiver(codecType, pc.dtlsTransport)
			if err != nil {
				pc.log.Warnf("Could not create RTPReceiver %s", err)
				return
			}

			if err = receiver.Receive(RTPReceiveParameters{
				Encodings: RTPDecodingParameters{
					RTPCodingParameters{SSRC: ssrc},
				}}); err != nil {
				pc.log.Warnf("RTPReceiver Receive failed %s", err)
				return
			}

			pc.newRTPTransceiver(
				receiver,
				nil,
				RTPTransceiverDirectionRecvonly,
			)

			if err = receiver.Track().determinePayloadType(); err != nil {
				pc.log.Warnf("Could not determine PayloadType for SSRC %d", receiver.Track().SSRC())
				return
			}

			pc.mu.RLock()
			defer pc.mu.RUnlock()

			sdpCodec, err := pc.currentLocalDescription.parsed.GetCodecForPayloadType(receiver.Track().PayloadType())
			if err != nil {
				pc.log.Warnf("no codec could be found in RemoteDescription for payloadType %d", receiver.Track().PayloadType())
				return
			}

			codec, err := pc.api.mediaEngine.getCodecSDP(sdpCodec)
			if err != nil {
				pc.log.Warnf("codec %s in not registered", sdpCodec)
				return
			}

			receiver.Track().mu.Lock()
			receiver.Track().kind = codec.Type
			receiver.Track().codec = codec
			receiver.Track().mu.Unlock()

			if pc.onTrackHandler != nil {
				pc.onTrack(receiver.Track(), receiver)
			} else {
				pc.log.Warnf("OnTrack unset, unable to handle incoming media streams")
			}
		}(i, incomingSSRCes[i])
	}
}

// drainSRTP pulls and discards RTP/RTCP packets that don't match any SRTP
// These could be sent to the user, but right now we don't provide an API
// to distribute orphaned RTCP messages. This is needed to make sure we don't block
// and provides useful debugging messages
func (pc *PeerConnection) drainSRTP() {
	go func() {
		for {
			srtpSession, err := pc.dtlsTransport.getSRTPSession()
			if err != nil {
				pc.log.Warnf("drainSRTP failed to open SrtpSession: %v", err)
				return
			}

			r, ssrc, err := srtpSession.AcceptStream()
			if err != nil {
				pc.log.Warnf("Failed to accept RTP %v \n", err)
				return
			}

			go func() {
				rtpBuf := make([]byte, receiveMTU)
				for {
					_, header, err := r.ReadRTP(rtpBuf)
					if err != nil {
						pc.log.Warnf("Failed to read, drainSRTP done for: %v %d \n", err, ssrc)
						return
					}

					pc.log.Debugf("got RTP: %+v", header)
				}
			}()
		}
	}()

	for {
		srtcpSession, err := pc.dtlsTransport.getSRTCPSession()
		if err != nil {
			pc.log.Warnf("drainSRTP failed to open SrtcpSession: %v", err)
			return
		}

		r, ssrc, err := srtcpSession.AcceptStream()
		if err != nil {
			pc.log.Warnf("Failed to accept RTCP %v \n", err)
			return
		}

		go func() {
			rtcpBuf := make([]byte, receiveMTU)
			for {
				_, header, err := r.ReadRTCP(rtcpBuf)
				if err != nil {
					pc.log.Warnf("Failed to read, drainSRTCP done for: %v %d \n", err, ssrc)
					return
				}
				pc.log.Debugf("got RTCP: %+v", header)
			}
		}()
	}
}

// RemoteDescription returns pendingRemoteDescription if it is not null and
// otherwise it returns currentRemoteDescription. This property is used to
// determine if setRemoteDescription has already been called.
// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-remotedescription
func (pc *PeerConnection) RemoteDescription() *SessionDescription {
	if pc.pendingRemoteDescription != nil {
		return pc.pendingRemoteDescription
	}
	return pc.currentRemoteDescription
}

// AddICECandidate accepts an ICE candidate string and adds it
// to the existing set of candidates
func (pc *PeerConnection) AddICECandidate(candidate ICECandidateInit) error {
	if pc.RemoteDescription() == nil {
		return &rtcerr.InvalidStateError{Err: ErrNoRemoteDescription}
	}

	candidateValue := strings.TrimPrefix(candidate.Candidate, "candidate:")
	attribute := sdp.NewAttribute("candidate", candidateValue)
	sdpCandidate, err := attribute.ToICECandidate()
	if err != nil {
		return err
	}

	iceCandidate, err := newICECandidateFromSDP(sdpCandidate)
	if err != nil {
		return err
	}

	return pc.iceTransport.AddRemoteCandidate(iceCandidate)
}

// ICEConnectionState returns the ICE connection state of the
// PeerConnection instance.
func (pc *PeerConnection) ICEConnectionState() ICEConnectionState {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return pc.iceConnectionState
}

// ------------------------------------------------------------------------
// --- FIXME - BELOW CODE NEEDS RE-ORGANIZATION - https://w3c.github.io/webrtc-pc/#rtp-media-api
// ------------------------------------------------------------------------

// GetSenders returns the RTPSender that are currently attached to this PeerConnection
func (pc *PeerConnection) GetSenders() []*RTPSender {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	result := []*RTPSender{}
	for _, tranceiver := range pc.rtpTransceivers {
		if tranceiver.Sender != nil {
			result = append(result, tranceiver.Sender)
		}
	}
	return result
}

// GetReceivers returns the RTPReceivers that are currently attached to this RTCPeerConnection
func (pc *PeerConnection) GetReceivers() []*RTPReceiver {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	result := []*RTPReceiver{}
	for _, tranceiver := range pc.rtpTransceivers {
		if tranceiver.Receiver != nil {
			result = append(result, tranceiver.Receiver)
		}
	}
	return result
}

// GetTransceivers returns the RTCRtpTransceiver that are currently attached to this RTCPeerConnection
func (pc *PeerConnection) GetTransceivers() []*RTPTransceiver {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	return pc.rtpTransceivers
}

// AddTrack adds a Track to the PeerConnection
func (pc *PeerConnection) AddTrack(track *Track) (*RTPSender, error) {
	if pc.isClosed {
		return nil, &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}
	var transceiver *RTPTransceiver
	for _, t := range pc.rtpTransceivers {
		if !t.stopped &&
			t.Sender != nil &&
			!t.Sender.hasSent() &&
			t.Receiver != nil &&
			t.Receiver.Track() != nil &&
			t.Receiver.Track().Kind() == track.Kind() {
			transceiver = t
			break
		}
	}
	if transceiver != nil {
		if err := transceiver.setSendingTrack(track); err != nil {
			return nil, err
		}
	} else {
		sender, err := pc.api.NewRTPSender(track, pc.dtlsTransport)
		if err != nil {
			return nil, err
		}
		transceiver = pc.newRTPTransceiver(
			nil,
			sender,
			RTPTransceiverDirectionSendonly,
		)
	}

	transceiver.Mid = track.Kind().String() // TODO: Mid generation

	return transceiver.Sender, nil
}

// func (pc *PeerConnection) RemoveTrack() {
// 	panic("not implemented yet") // FIXME NOT-IMPLEMENTED nolint
// }

// func (pc *PeerConnection) AddTransceiver() RTPTransceiver {
// 	panic("not implemented yet") // FIXME NOT-IMPLEMENTED nolint
// }

// CreateDataChannel creates a new DataChannel object with the given label
// and optional DataChannelInit used to configure properties of the
// underlying channel such as data reliability.
func (pc *PeerConnection) CreateDataChannel(label string, options *DataChannelInit) (*DataChannel, error) {
	// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #2)
	if pc.isClosed {
		return nil, &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	// TODO: Add additional options once implemented. DataChannelInit
	// implements all options. DataChannelParameters implements the
	// options that actually have an effect at this point.
	params := &DataChannelParameters{
		Label:   label,
		Ordered: true,
	}

	// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #19)
	if options == nil || options.ID == nil {
		var err error
		if params.ID, err = pc.generateDataChannelID(true); err != nil {
			return nil, err
		}
	} else {
		params.ID = *options.ID
	}

	if options != nil {
		// Ordered indicates if data is allowed to be delivered out of order. The
		// default value of true, guarantees that data will be delivered in order.
		if options.Ordered != nil {
			params.Ordered = *options.Ordered
		}

		// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #7)
		if options.MaxPacketLifeTime != nil {
			params.MaxPacketLifeTime = options.MaxPacketLifeTime
		}

		// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #8)
		if options.MaxRetransmits != nil {
			params.MaxRetransmits = options.MaxRetransmits
		}

		// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #9)
		if options.Ordered != nil {
			params.Ordered = *options.Ordered
		}
	}

	// TODO: Enable validation of other parameters once they are implemented.
	// - Protocol
	// - Negotiated
	// - Priority:
	//
	// See https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api for details

	d, err := pc.api.newDataChannel(params, pc.log)
	if err != nil {
		return nil, err
	}

	// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #16)
	if d.maxPacketLifeTime != nil && d.maxRetransmits != nil {
		return nil, &rtcerr.TypeError{Err: ErrRetransmitsOrPacketLifeTime}
	}

	// Remember datachannel
	pc.dataChannels[params.ID] = d

	// Open if networking already started
	if pc.sctpTransport != nil {
		err = d.open(pc.sctpTransport)
		if err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (pc *PeerConnection) generateDataChannelID(client bool) (uint16, error) {
	var id uint16
	if !client {
		id++
	}

	max := sctpMaxChannels
	if pc.sctpTransport != nil {
		max = *pc.sctpTransport.MaxChannels
	}

	for ; id < max-1; id += 2 {
		_, ok := pc.dataChannels[id]
		if !ok {
			return id, nil
		}
	}
	return 0, &rtcerr.OperationError{Err: ErrMaxDataChannelID}
}

// SetIdentityProvider is used to configure an identity provider to generate identity assertions
func (pc *PeerConnection) SetIdentityProvider(provider string) error {
	return fmt.Errorf("TODO SetIdentityProvider")
}

// SendRTCP sends a user provided RTCP packet to the connected peer
// If no peer is connected the packet is discarded
func (pc *PeerConnection) SendRTCP(pkt rtcp.Packet) error {
	raw, err := pkt.Marshal()
	if err != nil {
		return err
	}

	srtcpSession, err := pc.dtlsTransport.getSRTCPSession()
	if err != nil {
		return nil // TODO SendRTCP before would gracefully discard packets until ready
	}

	writeStream, err := srtcpSession.OpenWriteStream()
	if err != nil {
		return fmt.Errorf("SendRTCP failed to open WriteStream: %v", err)
	}

	if _, err := writeStream.Write(raw); err != nil {
		if err == ice.ErrNoCandidatePairs {
			return nil
		}
		return fmt.Errorf("SendRTCP failed to write: %v", err)
	}
	return nil
}

// Close ends the PeerConnection
func (pc *PeerConnection) Close() error {
	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #2)
	if pc.isClosed {
		return nil
	}

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #3)
	pc.isClosed = true

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #4)
	pc.signalingState = SignalingStateClosed

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #11)
	// pc.ICEConnectionState = ICEConnectionStateClosed
	pc.iceStateChange(ice.ConnectionStateClosed) // FIXME REMOVE

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #12)
	pc.connectionState = PeerConnectionStateClosed

	// Try closing everything and collect the errors
	var closeErrs []error

	// Shutdown strategy:
	// 1. All Conn close by closing their underlying Conn.
	// 2. A Mux stops this chain. It won't close the underlying
	//    Conn if one of the endpoints is closed down. To
	//    continue the chain the Mux has to be closed.

	if pc.iceTransport != nil {
		if err := pc.iceTransport.Stop(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}

	if err := pc.dtlsTransport.Stop(); err != nil {
		closeErrs = append(closeErrs, err)
	}

	if pc.sctpTransport != nil {
		if err := pc.sctpTransport.Stop(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}

	for _, t := range pc.rtpTransceivers {
		if err := t.Stop(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}

	// TODO: Figure out stopping ICE transport & Gatherer independently.
	// pc.iceGatherer()
	return util.FlattenErrs(closeErrs)
}

func (pc *PeerConnection) iceStateChange(newState ICEConnectionState) {
	pc.mu.Lock()
	pc.iceConnectionState = newState
	pc.mu.Unlock()

	pc.onICEConnectionStateChange(newState)
}

func localDirection(weSend bool, peerDirection RTPTransceiverDirection) RTPTransceiverDirection {
	theySend := (peerDirection == RTPTransceiverDirectionSendrecv || peerDirection == RTPTransceiverDirectionSendonly)
	switch {
	case weSend && theySend:
		return RTPTransceiverDirectionSendrecv
	case weSend && !theySend:
		return RTPTransceiverDirectionSendonly
	case !weSend && theySend:
		return RTPTransceiverDirectionRecvonly
	}

	return RTPTransceiverDirectionInactive
}

func (pc *PeerConnection) addFingerprint(d *sdp.SessionDescription) {
	// TODO: Handle multiple certificates
	for _, fingerprint := range pc.configuration.Certificates[0].GetFingerprints() {
		d.WithFingerprint(fingerprint.Algorithm, strings.ToUpper(fingerprint.Value))
	}
}

func (pc *PeerConnection) addRTPMediaSection(d *sdp.SessionDescription, codecType RTPCodecType, midValue string, iceParams ICEParameters, peerDirection RTPTransceiverDirection, candidates []ICECandidate, dtlsRole sdp.ConnectionRole) bool {
	if codecs := pc.api.mediaEngine.getCodecsByKind(codecType); len(codecs) == 0 {
		d.WithMedia(&sdp.MediaDescription{
			MediaName: sdp.MediaName{
				Media:   codecType.String(),
				Port:    sdp.RangedPort{Value: 0},
				Protos:  []string{"UDP", "TLS", "RTP", "SAVPF"},
				Formats: []string{"0"},
			},
		})
		return false
	}
	media := sdp.NewJSEPMediaDescription(codecType.String(), []string{}).
		WithValueAttribute(sdp.AttrKeyConnectionSetup, dtlsRole.String()). // TODO: Support other connection types
		WithValueAttribute(sdp.AttrKeyMID, midValue).
		WithICECredentials(iceParams.UsernameFragment, iceParams.Password).
		WithPropertyAttribute(sdp.AttrKeyRTCPMux).  // TODO: support RTCP fallback
		WithPropertyAttribute(sdp.AttrKeyRTCPRsize) // TODO: Support Reduced-Size RTCP?

	for _, codec := range pc.api.mediaEngine.getCodecsByKind(codecType) {
		media.WithCodec(codec.PayloadType, codec.Name, codec.ClockRate, codec.Channels, codec.SDPFmtpLine)

		for _, feedback := range codec.RTPCodecCapability.RTCPFeedback {
			media.WithValueAttribute("rtcp-fb", fmt.Sprintf("%d %s %s", codec.PayloadType, feedback.Type, feedback.Parameter))
		}
	}

	weSend := false
	for _, transceiver := range pc.rtpTransceivers {
		if transceiver.Sender == nil ||
			transceiver.Sender.track == nil ||
			transceiver.Sender.track.Kind() != codecType {
			continue
		}
		weSend = true
		track := transceiver.Sender.track
		media = media.WithMediaSource(track.SSRC(), track.Label() /* cname */, track.Label() /* streamLabel */, track.Label())
	}
	media = media.WithPropertyAttribute(localDirection(weSend, peerDirection).String())

	for _, c := range candidates {
		sdpCandidate := c.toSDP()
		sdpCandidate.ExtensionAttributes = append(sdpCandidate.ExtensionAttributes, sdp.ICECandidateAttribute{Key: "generation", Value: "0"})
		sdpCandidate.Component = 1
		media.WithICECandidate(sdpCandidate)
		sdpCandidate.Component = 2
		media.WithICECandidate(sdpCandidate)
	}
	media.WithPropertyAttribute("end-of-candidates")
	d.WithMedia(media)
	return true
}

func (pc *PeerConnection) addDataMediaSection(d *sdp.SessionDescription, midValue string, iceParams ICEParameters, candidates []ICECandidate, dtlsRole sdp.ConnectionRole) {
	media := (&sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media:   "application",
			Port:    sdp.RangedPort{Value: 9},
			Protos:  []string{"DTLS", "SCTP"},
			Formats: []string{"5000"},
		},
		ConnectionInformation: &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address: &sdp.Address{
				IP: net.ParseIP("0.0.0.0"),
			},
		},
	}).
		WithValueAttribute(sdp.AttrKeyConnectionSetup, dtlsRole.String()). // TODO: Support other connection types
		WithValueAttribute(sdp.AttrKeyMID, midValue).
		WithPropertyAttribute(RTPTransceiverDirectionSendrecv.String()).
		WithPropertyAttribute("sctpmap:5000 webrtc-datachannel 1024").
		WithICECredentials(iceParams.UsernameFragment, iceParams.Password)

	for _, c := range candidates {
		sdpCandidate := c.toSDP()
		sdpCandidate.ExtensionAttributes = append(sdpCandidate.ExtensionAttributes, sdp.ICECandidateAttribute{Key: "generation", Value: "0"})
		sdpCandidate.Component = 1
		media.WithICECandidate(sdpCandidate)
		sdpCandidate.Component = 2
		media.WithICECandidate(sdpCandidate)
	}
	media.WithPropertyAttribute("end-of-candidates")

	d.WithMedia(media)
}

// NewTrack Creates a new Track
func (pc *PeerConnection) NewTrack(payloadType uint8, ssrc uint32, id, label string) (*Track, error) {
	codec, err := pc.api.mediaEngine.getCodec(payloadType)
	if err != nil {
		return nil, err
	} else if codec.Payloader == nil {
		return nil, fmt.Errorf("codec payloader not set")
	}

	return NewTrack(payloadType, ssrc, id, label, codec)
}

func (pc *PeerConnection) newRTPTransceiver(
	receiver *RTPReceiver,
	sender *RTPSender,
	direction RTPTransceiverDirection,
) *RTPTransceiver {

	t := &RTPTransceiver{
		Receiver:  receiver,
		Sender:    sender,
		Direction: direction,
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.rtpTransceivers = append(pc.rtpTransceivers, t)
	return t
}

// CurrentLocalDescription represents the local description that was
// successfully negotiated the last time the PeerConnection transitioned
// into the stable state plus any local candidates that have been generated
// by the ICEAgent since the offer or answer was created.
func (pc *PeerConnection) CurrentLocalDescription() *SessionDescription {
	return pc.currentLocalDescription
}

// PendingLocalDescription represents a local description that is in the
// process of being negotiated plus any local candidates that have been
// generated by the ICEAgent since the offer or answer was created. If the
// PeerConnection is in the stable state, the value is null.
func (pc *PeerConnection) PendingLocalDescription() *SessionDescription {
	return pc.pendingLocalDescription
}

// CurrentRemoteDescription represents the last remote description that was
// successfully negotiated the last time the PeerConnection transitioned
// into the stable state plus any remote candidates that have been supplied
// via AddICECandidate() since the offer or answer was created.
func (pc *PeerConnection) CurrentRemoteDescription() *SessionDescription {
	return pc.currentRemoteDescription
}

// PendingRemoteDescription represents a remote description that is in the
// process of being negotiated, complete with any remote candidates that
// have been supplied via AddICECandidate() since the offer or answer was
// created. If the PeerConnection is in the stable state, the value is
// null.
func (pc *PeerConnection) PendingRemoteDescription() *SessionDescription {
	return pc.pendingRemoteDescription
}

// SignalingState attribute returns the signaling state of the
// PeerConnection instance.
func (pc *PeerConnection) SignalingState() SignalingState {
	return pc.signalingState
}

// ICEGatheringState attribute returns the ICE gathering state of the
// PeerConnection instance.
func (pc *PeerConnection) ICEGatheringState() ICEGatheringState {
	return pc.iceGatheringState
}

// ConnectionState attribute returns the connection state of the
// PeerConnection instance.
func (pc *PeerConnection) ConnectionState() PeerConnectionState {
	return pc.connectionState
}
