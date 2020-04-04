// +build !js

// Package webrtc implements the WebRTC 1.0 as defined in W3C WebRTC specification document.
package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	mathRand "math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/sdp/v2"

	"github.com/pion/webrtc/v2/internal/util"
	"github.com/pion/webrtc/v2/pkg/rtcerr"
)

// PeerConnection represents a WebRTC connection that establishes a
// peer-to-peer communications with another PeerConnection instance in a
// browser, or to another endpoint implementing the required protocols.
type PeerConnection struct {
	statsID string
	mu      sync.RWMutex

	// Negotiations must be processed serially
	negotationLock sync.Mutex

	configuration Configuration

	currentLocalDescription  *SessionDescription
	pendingLocalDescription  *SessionDescription
	currentRemoteDescription *SessionDescription
	pendingRemoteDescription *SessionDescription
	signalingState           SignalingState
	iceConnectionState       ICEConnectionState
	connectionState          PeerConnectionState

	idpLoginURL *string

	isClosed                     *atomicBool
	negotiationNeeded            bool
	nonTrickleCandidatesSignaled *atomicBool

	lastOffer  string
	lastAnswer string

	rtpTransceivers []*RTPTransceiver

	onSignalingStateChangeHandler     func(SignalingState)
	onICEConnectionStateChangeHandler func(ICEConnectionState)
	onConnectionStateChangeHandler    func(PeerConnectionState)
	onTrackHandler                    func(*Track, *RTPReceiver)
	onDataChannelHandler              func(*DataChannel)

	iceGatherer   *ICEGatherer
	iceTransport  *ICETransport
	dtlsTransport *DTLSTransport
	sctpTransport *SCTPTransport

	// A reference to the associated API state used by this connection
	api *API
	log logging.LeveledLogger
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
		statsID: fmt.Sprintf("PeerConnection-%d", time.Now().UnixNano()),
		configuration: Configuration{
			ICEServers:           []ICEServer{},
			ICETransportPolicy:   ICETransportPolicyAll,
			BundlePolicy:         BundlePolicyBalanced,
			RTCPMuxPolicy:        RTCPMuxPolicyRequire,
			Certificates:         []Certificate{},
			ICECandidatePoolSize: 0,
		},
		isClosed:                     &atomicBool{},
		negotiationNeeded:            false,
		nonTrickleCandidatesSignaled: &atomicBool{},
		lastOffer:                    "",
		lastAnswer:                   "",
		signalingState:               SignalingStateStable,
		iceConnectionState:           ICEConnectionStateNew,
		connectionState:              PeerConnectionStateNew,

		api: api,
		log: api.settingEngine.LoggerFactory.NewLogger("pc"),
	}

	var err error
	if err = pc.initConfiguration(configuration); err != nil {
		return nil, err
	}

	pc.iceGatherer, err = pc.createICEGatherer()
	if err != nil {
		return nil, err
	}

	if !pc.api.settingEngine.candidates.ICETrickle {
		if err = pc.iceGatherer.Gather(); err != nil {
			return nil, err
		}
	}

	// Create the ice transport
	iceTransport := pc.createICETransport()
	pc.iceTransport = iceTransport

	// Create the DTLS transport
	dtlsTransport, err := pc.api.NewDTLSTransport(pc.iceTransport, pc.configuration.Certificates)
	if err != nil {
		return nil, err
	}
	pc.dtlsTransport = dtlsTransport

	// Create the SCTP transport
	pc.sctpTransport = pc.api.NewSCTPTransport(pc.dtlsTransport)

	// Wire up the on datachannel handler
	pc.sctpTransport.OnDataChannel(func(d *DataChannel) {
		pc.mu.RLock()
		hdlr := pc.onDataChannelHandler
		pc.mu.RUnlock()
		if hdlr != nil {
			hdlr(d)
		}
	})

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

	if configuration.SDPSemantics != SDPSemantics(Unknown) {
		pc.configuration.SDPSemantics = configuration.SDPSemantics
	}

	sanitizedICEServers := configuration.getICEServers()
	if len(sanitizedICEServers) > 0 {
		for _, server := range sanitizedICEServers {
			if err := server.validate(); err != nil {
				return err
			}
		}
		pc.configuration.ICEServers = sanitizedICEServers
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

func (pc *PeerConnection) onSignalingStateChange(newState SignalingState) {
	pc.mu.RLock()
	hdlr := pc.onSignalingStateChangeHandler
	pc.mu.RUnlock()

	pc.log.Infof("signaling state changed to %s", newState)
	if hdlr != nil {
		go hdlr(newState)
	}
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
// Take note that the handler is gonna be called with a nil pointer when
// gathering is finished.
func (pc *PeerConnection) OnICECandidate(f func(*ICECandidate)) {
	pc.iceGatherer.OnLocalCandidate(f)
}

// OnICEGatheringStateChange sets an event handler which is invoked when the
// ICE candidate gathering state has changed.
func (pc *PeerConnection) OnICEGatheringStateChange(f func(ICEGathererState)) {
	pc.iceGatherer.OnStateChange(f)
}

// OnTrack sets an event handler which is called when remote track
// arrives from a remote peer.
func (pc *PeerConnection) OnTrack(f func(*Track, *RTPReceiver)) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.onTrackHandler = f
}

func (pc *PeerConnection) onTrack(t *Track, r *RTPReceiver) {
	pc.mu.RLock()
	hdlr := pc.onTrackHandler
	pc.mu.RUnlock()

	pc.log.Debugf("got new track: %+v", t)
	if hdlr != nil && t != nil {
		go hdlr(t, r)
	}
}

// OnICEConnectionStateChange sets an event handler which is called
// when an ICE connection state is changed.
func (pc *PeerConnection) OnICEConnectionStateChange(f func(ICEConnectionState)) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.onICEConnectionStateChangeHandler = f
}

func (pc *PeerConnection) onICEConnectionStateChange(cs ICEConnectionState) {
	pc.mu.Lock()
	pc.iceConnectionState = cs
	hdlr := pc.onICEConnectionStateChangeHandler
	pc.mu.Unlock()

	pc.log.Infof("ICE connection state changed: %s", cs)
	if hdlr != nil {
		go hdlr(cs)
	}
}

// OnConnectionStateChange sets an event handler which is called
// when the PeerConnectionState has changed
func (pc *PeerConnection) OnConnectionStateChange(f func(PeerConnectionState)) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.onConnectionStateChangeHandler = f
}

// SetConfiguration updates the configuration of this PeerConnection object.
func (pc *PeerConnection) SetConfiguration(configuration Configuration) error {
	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-setconfiguration (step #2)
	if pc.isClosed.get() {
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
			if err := server.validate(); err != nil {
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

func (pc *PeerConnection) getStatsID() string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.statsID
}

// CreateOffer starts the PeerConnection and generates the localDescription
func (pc *PeerConnection) CreateOffer(options *OfferOptions) (SessionDescription, error) {
	useIdentity := pc.idpLoginURL != nil
	switch {
	case options != nil:
		return SessionDescription{}, fmt.Errorf("TODO handle options")
	case useIdentity:
		return SessionDescription{}, fmt.Errorf("TODO handle identity provider")
	case pc.isClosed.get():
		return SessionDescription{}, &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	var (
		d   *sdp.SessionDescription
		err error
	)

	if pc.currentRemoteDescription == nil {
		d, err = pc.generateUnmatchedSDP(useIdentity)
	} else {
		d, err = pc.generateMatchedSDP(useIdentity, true /*includeUnmatched */, connectionRoleFromDtlsRole(defaultDtlsRoleOffer))
	}
	if err != nil {
		return SessionDescription{}, err
	}

	sdpBytes, err := d.Marshal()
	if err != nil {
		return SessionDescription{}, err
	}

	desc := SessionDescription{
		Type:   SDPTypeOffer,
		SDP:    string(sdpBytes),
		parsed: d,
	}
	pc.lastOffer = desc.SDP
	return desc, nil
}

func (pc *PeerConnection) createICEGatherer() (*ICEGatherer, error) {
	g, err := pc.api.NewICEGatherer(ICEGatherOptions{
		ICEServers:      pc.configuration.getICEServers(),
		ICEGatherPolicy: pc.configuration.ICETransportPolicy,
	})
	if err != nil {
		return nil, err
	}

	return g, nil
}

// Update the PeerConnectionState given the state of relevant transports
// https://www.w3.org/TR/webrtc/#rtcpeerconnectionstate-enum
func (pc *PeerConnection) updateConnectionState(iceConnectionState ICEConnectionState, dtlsTransportState DTLSTransportState) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	connectionState := PeerConnectionStateNew
	switch {
	// The RTCPeerConnection object's [[IsClosed]] slot is true.
	case pc.isClosed.get():
		connectionState = PeerConnectionStateClosed

	// Any of the RTCIceTransports or RTCDtlsTransports are in a "failed" state.
	case iceConnectionState == ICEConnectionStateFailed || dtlsTransportState == DTLSTransportStateFailed:
		connectionState = PeerConnectionStateFailed

	// Any of the RTCIceTransports or RTCDtlsTransports are in the "disconnected"
	// state and none of them are in the "failed" or "connecting" or "checking" state.  */
	case iceConnectionState == ICEConnectionStateDisconnected:
		connectionState = PeerConnectionStateDisconnected

	// All RTCIceTransports and RTCDtlsTransports are in the "connected", "completed" or "closed"
	// state and at least one of them is in the "connected" or "completed" state.
	case iceConnectionState == ICEConnectionStateConnected && dtlsTransportState == DTLSTransportStateConnected:
		connectionState = PeerConnectionStateConnected

	//  Any of the RTCIceTransports or RTCDtlsTransports are in the "connecting" or
	// "checking" state and none of them is in the "failed" state.
	case iceConnectionState == ICEConnectionStateChecking && dtlsTransportState == DTLSTransportStateConnecting:
		connectionState = PeerConnectionStateConnecting
	}

	if pc.connectionState == connectionState {
		return
	}

	pc.log.Infof("peer connection state changed: %s", connectionState)
	pc.connectionState = connectionState
	hdlr := pc.onConnectionStateChangeHandler
	if hdlr != nil {
		go hdlr(connectionState)
	}
}

func (pc *PeerConnection) createICETransport() *ICETransport {
	t := pc.api.NewICETransport(pc.iceGatherer)
	t.OnConnectionStateChange(func(state ICETransportState) {
		var cs ICEConnectionState
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
		pc.onICEConnectionStateChange(cs)
		pc.updateConnectionState(cs, pc.dtlsTransport.State())
	})

	return t
}

// CreateAnswer starts the PeerConnection and generates the localDescription
func (pc *PeerConnection) CreateAnswer(options *AnswerOptions) (SessionDescription, error) {
	useIdentity := pc.idpLoginURL != nil
	switch {
	case options != nil:
		return SessionDescription{}, fmt.Errorf("TODO handle options")
	case pc.RemoteDescription() == nil:
		return SessionDescription{}, &rtcerr.InvalidStateError{Err: ErrNoRemoteDescription}
	case useIdentity:
		return SessionDescription{}, fmt.Errorf("TODO handle identity provider")
	case pc.isClosed.get():
		return SessionDescription{}, &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	connectionRole := connectionRoleFromDtlsRole(pc.api.settingEngine.answeringDTLSRole)
	if connectionRole == sdp.ConnectionRole(0) {
		connectionRole = connectionRoleFromDtlsRole(defaultDtlsRoleAnswer)
	}

	d, err := pc.generateMatchedSDP(useIdentity, false /*includeUnmatched */, connectionRole)
	if err != nil {
		return SessionDescription{}, err
	}

	sdpBytes, err := d.Marshal()
	if err != nil {
		return SessionDescription{}, err
	}

	desc := SessionDescription{
		Type:   SDPTypeAnswer,
		SDP:    string(sdpBytes),
		parsed: d,
	}
	pc.lastAnswer = desc.SDP
	return desc, nil
}

// 4.4.1.6 Set the SessionDescription
func (pc *PeerConnection) setDescription(sd *SessionDescription, op stateChangeOp) error {
	if pc.isClosed.get() {
		return &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	nextState, err := func() (SignalingState, error) {
		pc.mu.Lock()
		defer pc.mu.Unlock()

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
					return nextState, newSDPDoesNotMatchOffer
				}
				nextState, err = checkNextSignalingState(cur, SignalingStateHaveLocalOffer, setLocal, sd.Type)
				if err == nil {
					pc.pendingLocalDescription = sd
				}
			// have-remote-offer->SetLocal(answer)->stable
			// have-local-pranswer->SetLocal(answer)->stable
			case SDPTypeAnswer:
				if sd.SDP != pc.lastAnswer {
					return nextState, newSDPDoesNotMatchAnswer
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
					return nextState, newSDPDoesNotMatchAnswer
				}
				nextState, err = checkNextSignalingState(cur, SignalingStateHaveLocalPranswer, setLocal, sd.Type)
				if err == nil {
					pc.pendingLocalDescription = sd
				}
			default:
				return nextState, &rtcerr.OperationError{Err: fmt.Errorf("invalid state change op: %s(%s)", op, sd.Type)}
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
				return nextState, &rtcerr.OperationError{Err: fmt.Errorf("invalid state change op: %s(%s)", op, sd.Type)}
			}
		default:
			return nextState, &rtcerr.OperationError{Err: fmt.Errorf("unhandled state change op: %q", op)}
		}

		return nextState, nil
	}()

	if err == nil {
		pc.signalingState = nextState
		pc.onSignalingStateChange(nextState)
	}
	return err
}

// SetLocalDescription sets the SessionDescription of the local peer
func (pc *PeerConnection) SetLocalDescription(desc SessionDescription) error {
	if pc.isClosed.get() {
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

	desc.parsed = &sdp.SessionDescription{}
	if err := desc.parsed.Unmarshal([]byte(desc.SDP)); err != nil {
		return err
	}
	if err := pc.setDescription(&desc, stateChangeOpSetLocal); err != nil {
		return err
	}

	// To support all unittests which are following the future trickle=true
	// setup while also support the old trickle=false synchronous gathering
	// process this is necessary to avoid calling Gather() in multiple
	// places; which causes race conditions. (issue-707)
	if !pc.api.settingEngine.candidates.ICETrickle && !pc.nonTrickleCandidatesSignaled.get() {
		if err := pc.iceGatherer.SignalCandidates(); err != nil {
			return err
		}
		pc.nonTrickleCandidatesSignaled.set(true)
		return nil
	}

	return pc.iceGatherer.Gather()
}

// LocalDescription returns pendingLocalDescription if it is not null and
// otherwise it returns currentLocalDescription. This property is used to
// determine if setLocalDescription has already been called.
// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-localdescription
func (pc *PeerConnection) LocalDescription() *SessionDescription {
	if localDescription := pc.PendingLocalDescription(); localDescription != nil {
		return localDescription
	}
	return pc.currentLocalDescription
}

// SetRemoteDescription sets the SessionDescription of the remote peer
func (pc *PeerConnection) SetRemoteDescription(desc SessionDescription) error {
	if pc.isClosed.get() {
		return &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	currentTransceivers := append([]*RTPTransceiver{}, pc.GetTransceivers()...)
	haveRemoteDescription := pc.currentRemoteDescription != nil

	desc.parsed = &sdp.SessionDescription{}
	if err := desc.parsed.Unmarshal([]byte(desc.SDP)); err != nil {
		return err
	}
	if err := pc.setDescription(&desc, stateChangeOpSetRemote); err != nil {
		return err
	}

	if haveRemoteDescription {
		pc.startRenegotation(currentTransceivers)
		return nil
	}

	weOffer := true
	if desc.Type == SDPTypeOffer {
		weOffer = false
	}

	remoteIsLite := false
	if liteValue, haveRemoteIs := desc.parsed.Attribute(sdp.AttrKeyICELite); haveRemoteIs && liteValue == sdp.AttrKeyICELite {
		remoteIsLite = true
	}

	fingerprint, fingerprintHash, err := extractFingerprint(desc.parsed)
	if err != nil {
		return err
	}

	remoteUfrag, remotePwd, candidates, err := extractICEDetails(desc.parsed)
	if err != nil {
		return err
	}

	for _, c := range candidates {
		if err = pc.iceTransport.AddRemoteCandidate(c); err != nil {
			return err
		}
	}

	iceRole := ICERoleControlled
	// If one of the agents is lite and the other one is not, the lite agent must be the controlling agent.
	// If both or neither agents are lite the offering agent is controlling.
	// RFC 8445 S6.1.1
	if (weOffer && remoteIsLite == pc.api.settingEngine.candidates.ICELite) || (remoteIsLite && !pc.api.settingEngine.candidates.ICELite) {
		iceRole = ICERoleControlling
	}

	// Start the networking in a new routine since it will block until
	// the connection is actually established.
	pc.startTransports(iceRole, dtlsRoleFromRemoteSDP(desc.parsed), remoteUfrag, remotePwd, fingerprint, fingerprintHash, currentTransceivers, trackDetailsFromSDP(pc.log, desc.parsed))
	return nil
}

func (pc *PeerConnection) startReceiver(incoming trackDetails, receiver *RTPReceiver) {
	err := receiver.Receive(RTPReceiveParameters{
		Encodings: RTPDecodingParameters{
			RTPCodingParameters{SSRC: incoming.ssrc},
		}})
	if err != nil {
		pc.log.Warnf("RTPReceiver Receive failed %s", err)
		return
	}

	if err = receiver.Track().determinePayloadType(); err != nil {
		pc.log.Warnf("Could not determine PayloadType for SSRC %d", receiver.Track().SSRC())
		return
	}

	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.currentLocalDescription == nil {
		pc.log.Warnf("SetLocalDescription not called, unable to handle incoming media streams")
		return
	}

	codec, err := pc.api.mediaEngine.getCodec(receiver.Track().PayloadType())
	if err != nil {
		pc.log.Warnf("no codec could be found for payloadType %d", receiver.Track().PayloadType())
		return
	}

	receiver.Track().mu.Lock()
	receiver.Track().id = incoming.id
	receiver.Track().label = incoming.label
	receiver.Track().kind = codec.Type
	receiver.Track().codec = codec
	receiver.Track().mu.Unlock()

	if pc.onTrackHandler != nil {
		pc.onTrack(receiver.Track(), receiver)
	} else {
		pc.log.Warnf("OnTrack unset, unable to handle incoming media streams")
	}
}

// startRTPReceivers opens knows inbound SRTP streams from the RemoteDescription
func (pc *PeerConnection) startRTPReceivers(incomingTracks map[uint32]trackDetails, currentTransceivers []*RTPTransceiver) {
	localTransceivers := append([]*RTPTransceiver{}, currentTransceivers...)

	remoteIsPlanB := false
	switch pc.configuration.SDPSemantics {
	case SDPSemanticsPlanB:
		remoteIsPlanB = true
	case SDPSemanticsUnifiedPlanWithFallback:
		remoteIsPlanB = descriptionIsPlanB(pc.RemoteDescription())
	}

	// Ensure we haven't already started a transceiver for this ssrc
	for ssrc := range incomingTracks {
		for i := range localTransceivers {
			if t := localTransceivers[i]; (t.Receiver()) == nil || t.Receiver().Track() == nil || t.Receiver().Track().ssrc != ssrc {
				continue
			}

			delete(incomingTracks, ssrc)
		}
	}

	for ssrc, incoming := range incomingTracks {
		for i := range localTransceivers {
			t := localTransceivers[i]

			if (incomingTracks[ssrc].kind != t.kind) ||
				(t.Direction() != RTPTransceiverDirectionRecvonly && t.Direction() != RTPTransceiverDirectionSendrecv) ||
				(t.Receiver()) == nil ||
				(t.Receiver().haveReceived()) {
				continue
			}

			delete(incomingTracks, ssrc)
			localTransceivers = append(localTransceivers[:i], localTransceivers[i+1:]...)
			go pc.startReceiver(incoming, t.Receiver())
			break
		}
	}

	if remoteIsPlanB {
		for ssrc, incoming := range incomingTracks {
			t, err := pc.AddTransceiverFromKind(incoming.kind, RtpTransceiverInit{
				Direction: RTPTransceiverDirectionSendrecv,
			})
			if err != nil {
				pc.log.Warnf("Could not add transceiver for remote SSRC %d: %s", ssrc, err)
				continue
			}
			go pc.startReceiver(incoming, t.Receiver())
		}
	}
}

// startRTPSenders starts all outbound RTP streams
func (pc *PeerConnection) startRTPSenders(currentTransceivers []*RTPTransceiver) {
	for _, tranceiver := range currentTransceivers {
		if tranceiver.Sender() != nil && !tranceiver.Sender().hasSent() {
			err := tranceiver.Sender().Send(RTPSendParameters{
				Encodings: RTPEncodingParameters{
					RTPCodingParameters{
						SSRC:        tranceiver.Sender().track.SSRC(),
						PayloadType: tranceiver.Sender().track.PayloadType(),
					},
				}})
			if err != nil {
				pc.log.Warnf("Failed to start Sender: %s", err)
			}
		}
	}
}

// Start SCTP subsystem
func (pc *PeerConnection) startSCTP() {
	// Start sctp
	if err := pc.sctpTransport.Start(SCTPCapabilities{
		MaxMessageSize: 0,
	}); err != nil {
		pc.log.Warnf("Failed to start SCTP: %s", err)
		if err = pc.sctpTransport.Stop(); err != nil {
			pc.log.Warnf("Failed to stop SCTPTransport: %s", err)
		}

		return
	}

	// DataChannels that need to be opened now that SCTP is available
	// make a copy we may have incoming DataChannels mutating this while we open
	pc.sctpTransport.lock.RLock()
	dataChannels := append([]*DataChannel{}, pc.sctpTransport.dataChannels...)
	pc.sctpTransport.lock.RUnlock()

	var openedDCCount uint32
	for _, d := range dataChannels {
		if d.ReadyState() == DataChannelStateConnecting {
			err := d.open(pc.sctpTransport)
			if err != nil {
				pc.log.Warnf("failed to open data channel: %s", err)
				continue
			}
			openedDCCount++
		}
	}

	pc.sctpTransport.lock.Lock()
	pc.sctpTransport.dataChannelsOpened += openedDCCount
	pc.sctpTransport.lock.Unlock()
}

// drainSRTP pulls and discards RTP/RTCP packets that don't match any a:ssrc lines
// If the remote SDP was only one media section the ssrc doesn't have to be explicitly declared
func (pc *PeerConnection) drainSRTP() {
	handleUndeclaredSSRC := func(ssrc uint32) bool {
		if remoteDescription := pc.RemoteDescription(); remoteDescription != nil {
			if len(remoteDescription.parsed.MediaDescriptions) == 1 {
				onlyMediaSection := remoteDescription.parsed.MediaDescriptions[0]
				for _, a := range onlyMediaSection.Attributes {
					if a.Key == ssrcStr {
						return false
					}
				}

				incoming := trackDetails{
					ssrc: ssrc,
					kind: RTPCodecTypeVideo,
				}
				if onlyMediaSection.MediaName.Media == RTPCodecTypeAudio.String() {
					incoming.kind = RTPCodecTypeAudio
				}

				t, err := pc.AddTransceiverFromKind(incoming.kind, RtpTransceiverInit{
					Direction: RTPTransceiverDirectionSendrecv,
				})
				if err != nil {
					pc.log.Warnf("Could not add transceiver for remote SSRC %d: %s", ssrc, err)
					return false
				}
				go pc.startReceiver(incoming, t.Receiver())
				return true
			}
		}

		return false
	}

	go func() {
		for {
			srtpSession, err := pc.dtlsTransport.getSRTPSession()
			if err != nil {
				pc.log.Warnf("drainSRTP failed to open SrtpSession: %v", err)
				return
			}

			_, ssrc, err := srtpSession.AcceptStream()
			if err != nil {
				pc.log.Warnf("Failed to accept RTP %v", err)
				return
			}

			if !handleUndeclaredSSRC(ssrc) {
				pc.log.Warnf("Incoming unhandled RTP ssrc(%d), OnTrack will not be fired", ssrc)
			}
		}
	}()

	go func() {
		for {
			srtcpSession, err := pc.dtlsTransport.getSRTCPSession()
			if err != nil {
				pc.log.Warnf("drainSRTP failed to open SrtcpSession: %v", err)
				return
			}

			_, ssrc, err := srtcpSession.AcceptStream()
			if err != nil {
				pc.log.Warnf("Failed to accept RTCP %v", err)
				return
			}
			pc.log.Warnf("Incoming unhandled RTCP ssrc(%d), OnTrack will not be fired", ssrc)
		}
	}()
}

// RemoteDescription returns pendingRemoteDescription if it is not null and
// otherwise it returns currentRemoteDescription. This property is used to
// determine if setRemoteDescription has already been called.
// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-remotedescription
func (pc *PeerConnection) RemoteDescription() *SessionDescription {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

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

// GetSenders returns the RTPSender that are currently attached to this PeerConnection
func (pc *PeerConnection) GetSenders() []*RTPSender {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	result := []*RTPSender{}
	for _, tranceiver := range pc.rtpTransceivers {
		if tranceiver.Sender() != nil {
			result = append(result, tranceiver.Sender())
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
		if tranceiver.Receiver() != nil {
			result = append(result, tranceiver.Receiver())
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
	if pc.isClosed.get() {
		return nil, &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	var transceiver *RTPTransceiver
	for _, t := range pc.GetTransceivers() {
		if !t.stopped && t.Sender() != nil &&
			!t.Sender().hasSent() &&
			t.Receiver() != nil &&
			t.Receiver().Track() != nil &&
			t.Receiver().Track().Kind() == track.Kind() {
			transceiver = t
			break
		}
	}
	if transceiver != nil {
		if err := transceiver.setSendingTrack(track); err != nil {
			return nil, err
		}
		return transceiver.Sender(), nil
	}

	transceiver, err := pc.AddTransceiverFromTrack(track)
	if err != nil {
		return nil, err
	}

	return transceiver.Sender(), nil
}

// AddTransceiver Create a new RTCRtpTransceiver and add it to the set of transceivers.
// Deprecated: Use AddTrack, AddTransceiverFromKind or AddTransceiverFromTrack
func (pc *PeerConnection) AddTransceiver(trackOrKind RTPCodecType, init ...RtpTransceiverInit) (*RTPTransceiver, error) {
	return pc.AddTransceiverFromKind(trackOrKind, init...)
}

// RemoveTrack removes a Track from the PeerConnection
func (pc *PeerConnection) RemoveTrack(sender *RTPSender) error {
	if pc.isClosed.get() {
		return &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	var transceiver *RTPTransceiver
	for _, t := range pc.GetTransceivers() {
		if t.Sender() == sender {
			transceiver = t
			break
		}
	}

	if transceiver == nil {
		return &rtcerr.InvalidAccessError{Err: ErrSenderNotCreatedByConnection}
	} else if err := sender.Stop(); err != nil {
		return err
	}

	return transceiver.setSendingTrack(nil)
}

// AddTransceiverFromKind Create a new RTCRtpTransceiver(SendRecv or RecvOnly) and add it to the set of transceivers.
func (pc *PeerConnection) AddTransceiverFromKind(kind RTPCodecType, init ...RtpTransceiverInit) (*RTPTransceiver, error) {
	if pc.isClosed.get() {
		return nil, &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	direction := RTPTransceiverDirectionSendrecv
	if len(init) > 1 {
		return nil, fmt.Errorf("AddTransceiverFromKind only accepts one RtpTransceiverInit")
	} else if len(init) == 1 {
		direction = init[0].Direction
	}

	switch direction {
	case RTPTransceiverDirectionSendrecv:
		codecs := pc.api.mediaEngine.GetCodecsByKind(kind)
		if len(codecs) == 0 {
			return nil, fmt.Errorf("no %s codecs found", kind.String())
		}

		track, err := pc.NewTrack(codecs[0].PayloadType, mathRand.Uint32(), util.RandSeq(trackDefaultIDLength), util.RandSeq(trackDefaultLabelLength))
		if err != nil {
			return nil, err
		}

		return pc.AddTransceiverFromTrack(track, init...)

	case RTPTransceiverDirectionRecvonly:
		receiver, err := pc.api.NewRTPReceiver(kind, pc.dtlsTransport)
		if err != nil {
			return nil, err
		}

		return pc.newRTPTransceiver(
			receiver,
			nil,
			RTPTransceiverDirectionRecvonly,
			kind,
		), nil
	default:
		return nil, fmt.Errorf("AddTransceiverFromKind currently only supports recvonly and sendrecv")
	}
}

// AddTransceiverFromTrack Creates a new send only transceiver and add it to the set of
func (pc *PeerConnection) AddTransceiverFromTrack(track *Track, init ...RtpTransceiverInit) (*RTPTransceiver, error) {
	if pc.isClosed.get() {
		return nil, &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	direction := RTPTransceiverDirectionSendrecv
	if len(init) > 1 {
		return nil, fmt.Errorf("AddTransceiverFromTrack only accepts one RtpTransceiverInit")
	} else if len(init) == 1 {
		direction = init[0].Direction
	}

	switch direction {
	case RTPTransceiverDirectionSendrecv:
		receiver, err := pc.api.NewRTPReceiver(track.Kind(), pc.dtlsTransport)
		if err != nil {
			return nil, err
		}

		sender, err := pc.api.NewRTPSender(track, pc.dtlsTransport)
		if err != nil {
			return nil, err
		}

		return pc.newRTPTransceiver(
			receiver,
			sender,
			RTPTransceiverDirectionSendrecv,
			track.Kind(),
		), nil

	case RTPTransceiverDirectionSendonly:
		sender, err := pc.api.NewRTPSender(track, pc.dtlsTransport)
		if err != nil {
			return nil, err
		}

		return pc.newRTPTransceiver(
			nil,
			sender,
			RTPTransceiverDirectionSendonly,
			track.Kind(),
		), nil
	default:
		return nil, fmt.Errorf("AddTransceiverFromTrack currently only supports sendonly and sendrecv")
	}
}

// CreateDataChannel creates a new DataChannel object with the given label
// and optional DataChannelInit used to configure properties of the
// underlying channel such as data reliability.
func (pc *PeerConnection) CreateDataChannel(label string, options *DataChannelInit) (*DataChannel, error) {
	// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #2)
	if pc.isClosed.get() {
		return nil, &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	params := &DataChannelParameters{
		Label:   label,
		Ordered: true,
	}

	// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #19)
	if options != nil {
		params.ID = options.ID
	}

	if options != nil {
		// Ordered indicates if data is allowed to be delivered out of order. The
		// default value of true, guarantees that data will be delivered in order.
		// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #9)
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

		// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #10)
		if options.Protocol != nil {
			params.Protocol = *options.Protocol
		}

		// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #11)
		if len(params.Protocol) > 65535 {
			return nil, &rtcerr.TypeError{Err: ErrProtocolTooLarge}
		}

		// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #12)
		if options.Negotiated != nil {
			params.Negotiated = *options.Negotiated
		}
	}

	d, err := pc.api.newDataChannel(params, pc.log)
	if err != nil {
		return nil, err
	}

	// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #16)
	if d.maxPacketLifeTime != nil && d.maxRetransmits != nil {
		return nil, &rtcerr.TypeError{Err: ErrRetransmitsOrPacketLifeTime}
	}

	pc.sctpTransport.lock.Lock()
	pc.sctpTransport.dataChannels = append(pc.sctpTransport.dataChannels, d)
	pc.sctpTransport.dataChannelsRequested++
	pc.sctpTransport.lock.Unlock()

	// If SCTP already connected open all the channels
	if pc.sctpTransport.State() == SCTPTransportStateConnected {
		if err = d.open(pc.sctpTransport); err != nil {
			return nil, err
		}
	}

	return d, nil
}

// SetIdentityProvider is used to configure an identity provider to generate identity assertions
func (pc *PeerConnection) SetIdentityProvider(provider string) error {
	return fmt.Errorf("TODO SetIdentityProvider")
}

// WriteRTCP sends a user provided RTCP packet to the connected peer
// If no peer is connected the packet is discarded
func (pc *PeerConnection) WriteRTCP(pkts []rtcp.Packet) error {
	raw, err := rtcp.Marshal(pkts)
	if err != nil {
		return err
	}

	srtcpSession, err := pc.dtlsTransport.getSRTCPSession()
	if err != nil {
		return nil
	}

	writeStream, err := srtcpSession.OpenWriteStream()
	if err != nil {
		return fmt.Errorf("WriteRTCP failed to open WriteStream: %v", err)
	}

	if _, err := writeStream.Write(raw); err != nil {
		return err
	}
	return nil
}

// Close ends the PeerConnection
func (pc *PeerConnection) Close() error {
	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #2)
	if pc.isClosed.get() {
		return nil
	}

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #3)
	pc.isClosed.set(true)

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #4)
	pc.signalingState = SignalingStateClosed

	// Try closing everything and collect the errors
	// Shutdown strategy:
	// 1. All Conn close by closing their underlying Conn.
	// 2. A Mux stops this chain. It won't close the underlying
	//    Conn if one of the endpoints is closed down. To
	//    continue the chain the Mux has to be closed.
	var closeErrs []error

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #5)
	for _, t := range pc.rtpTransceivers {
		if err := t.Stop(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #6)
	if pc.sctpTransport != nil {
		pc.sctpTransport.lock.Lock()
		for _, d := range pc.sctpTransport.dataChannels {
			d.setReadyState(DataChannelStateClosed)
		}
		pc.sctpTransport.lock.Unlock()
	}

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #7)
	if pc.sctpTransport != nil {
		if err := pc.sctpTransport.Stop(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #8)
	if err := pc.dtlsTransport.Stop(); err != nil {
		closeErrs = append(closeErrs, err)
	}

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #9,#10,#11)
	if pc.iceTransport != nil {
		if err := pc.iceTransport.Stop(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}

	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-close (step #12)
	pc.updateConnectionState(pc.ICEConnectionState(), pc.dtlsTransport.State())

	return util.FlattenErrs(closeErrs)
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
	kind RTPCodecType,
) *RTPTransceiver {
	t := &RTPTransceiver{kind: kind}
	t.setReceiver(receiver)
	t.setSender(sender)
	t.setDirection(direction)

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
	return populateLocalCandidates(pc.currentLocalDescription, pc.pendingLocalDescription, pc.iceGatherer, pc.ICEGatheringState())
}

// PendingLocalDescription represents a local description that is in the
// process of being negotiated plus any local candidates that have been
// generated by the ICEAgent since the offer or answer was created. If the
// PeerConnection is in the stable state, the value is null.
func (pc *PeerConnection) PendingLocalDescription() *SessionDescription {
	return populateLocalCandidates(pc.pendingLocalDescription, pc.pendingLocalDescription, pc.iceGatherer, pc.ICEGatheringState())
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
	if pc.iceGatherer == nil {
		return ICEGatheringStateNew
	}

	switch pc.iceGatherer.State() {
	case ICEGathererStateNew:
		return ICEGatheringStateNew
	case ICEGathererStateGathering:
		return ICEGatheringStateGathering
	default:
		return ICEGatheringStateComplete
	}
}

// ConnectionState attribute returns the connection state of the
// PeerConnection instance.
func (pc *PeerConnection) ConnectionState() PeerConnectionState {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	return pc.connectionState
}

// GetStats return data providing statistics about the overall connection
func (pc *PeerConnection) GetStats() StatsReport {
	var (
		dataChannelsAccepted  uint32
		dataChannelsClosed    uint32
		dataChannelsOpened    uint32
		dataChannelsRequested uint32
	)
	statsCollector := newStatsReportCollector()
	statsCollector.Collecting()

	pc.mu.Lock()
	if pc.iceGatherer != nil {
		pc.iceGatherer.collectStats(statsCollector)
	}
	if pc.iceTransport != nil {
		pc.iceTransport.collectStats(statsCollector)
	}

	if pc.sctpTransport != nil {
		pc.sctpTransport.lock.Lock()
		dataChannels := append([]*DataChannel{}, pc.sctpTransport.dataChannels...)
		dataChannelsAccepted = pc.sctpTransport.dataChannelsAccepted
		dataChannelsOpened = pc.sctpTransport.dataChannelsOpened
		dataChannelsRequested = pc.sctpTransport.dataChannelsRequested
		pc.sctpTransport.lock.Unlock()

		for _, d := range dataChannels {
			state := d.ReadyState()
			if state != DataChannelStateConnecting && state != DataChannelStateOpen {
				dataChannelsClosed++
			}

			d.collectStats(statsCollector)
		}
		pc.sctpTransport.collectStats(statsCollector)
	}

	stats := PeerConnectionStats{
		Timestamp:             statsTimestampNow(),
		Type:                  StatsTypePeerConnection,
		ID:                    pc.statsID,
		DataChannelsAccepted:  dataChannelsAccepted,
		DataChannelsClosed:    dataChannelsClosed,
		DataChannelsOpened:    dataChannelsOpened,
		DataChannelsRequested: dataChannelsRequested,
	}
	pc.mu.Unlock()

	statsCollector.Collect(stats.ID, stats)
	return statsCollector.Ready()
}

// Start all transports. PeerConnection now has enough state
func (pc *PeerConnection) startTransports(iceRole ICERole, dtlsRole DTLSRole, remoteUfrag, remotePwd, fingerprint, fingerprintHash string, currentTransceivers []*RTPTransceiver, incomingTracks map[uint32]trackDetails) {
	pc.negotationLock.Lock()

	go func() {
		defer pc.negotationLock.Unlock()

		// Start the ice transport
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
			pc.log.Warnf("Failed to start manager: %s", err)
			return
		}

		// Start the dtls transport
		err = pc.dtlsTransport.Start(DTLSParameters{
			Role:         dtlsRole,
			Fingerprints: []DTLSFingerprint{{Algorithm: fingerprintHash, Value: fingerprint}},
		})
		pc.updateConnectionState(pc.ICEConnectionState(), pc.dtlsTransport.State())
		if err != nil {
			pc.log.Warnf("Failed to start manager: %s", err)
			return
		}

		pc.startRTPReceivers(incomingTracks, currentTransceivers)
		pc.startRTPSenders(currentTransceivers)
		pc.drainSRTP()
		pc.startSCTP()
	}()
}

func (pc *PeerConnection) startRenegotation(currentTransceivers []*RTPTransceiver) {
	pc.negotationLock.Lock()

	go func() {
		defer pc.negotationLock.Unlock()

		trackDetails := trackDetailsFromSDP(pc.log, pc.RemoteDescription().parsed)
		for _, t := range currentTransceivers {
			if t.Receiver() == nil || t.Receiver().Track() == nil {
				continue
			} else if _, ok := trackDetails[t.Receiver().Track().ssrc]; ok {
				continue
			}

			if err := t.Receiver().Stop(); err != nil {
				pc.log.Warnf("Failed to stop RtpReceiver: %s", err)
				continue
			}

			receiver, err := pc.api.NewRTPReceiver(t.Receiver().kind, pc.dtlsTransport)
			if err != nil {
				pc.log.Warnf("Failed to create new RtpReceiver: %s", err)
				continue
			}
			t.setReceiver(receiver)
		}

		pc.startRTPSenders(currentTransceivers)
		pc.startRTPReceivers(trackDetails, currentTransceivers)
	}()
}

// GetRegisteredRTPCodecs gets a list of registered RTPCodec from the underlying constructed MediaEngine
func (pc *PeerConnection) GetRegisteredRTPCodecs(kind RTPCodecType) []*RTPCodec {
	return pc.api.mediaEngine.GetCodecsByKind(kind)
}

// generateUnmatchedSDP generates an SDP that doesn't take remote state into account
// This is used for the initial call for CreateOffer
func (pc *PeerConnection) generateUnmatchedSDP(useIdentity bool) (*sdp.SessionDescription, error) {
	d := sdp.NewJSEPSessionDescription(useIdentity)
	if err := addFingerprints(d, pc.configuration.Certificates[0]); err != nil {
		return nil, err
	}

	iceParams, err := pc.iceGatherer.GetLocalParameters()
	if err != nil {
		return nil, err
	}

	candidates, err := pc.iceGatherer.GetLocalCandidates()
	if err != nil {
		return nil, err
	}

	isPlanB := pc.configuration.SDPSemantics == SDPSemanticsPlanB
	mediaSections := []mediaSection{}

	if isPlanB {
		video := make([]*RTPTransceiver, 0)
		audio := make([]*RTPTransceiver, 0)

		for _, t := range pc.GetTransceivers() {
			if t.kind == RTPCodecTypeVideo {
				video = append(video, t)
			} else if t.kind == RTPCodecTypeAudio {
				audio = append(audio, t)
			}
		}

		if len(video) > 1 {
			mediaSections = append(mediaSections, mediaSection{id: "video", transceivers: video})
		}
		if len(audio) > 1 {
			mediaSections = append(mediaSections, mediaSection{id: "audio", transceivers: audio})
		}
		mediaSections = append(mediaSections, mediaSection{id: "data", data: true})
	} else {
		for _, t := range pc.GetTransceivers() {
			mediaSections = append(mediaSections, mediaSection{id: strconv.Itoa(len(mediaSections)), transceivers: []*RTPTransceiver{t}})
		}

		mediaSections = append(mediaSections, mediaSection{id: strconv.Itoa(len(mediaSections)), data: true})
	}

	return populateSDP(d, isPlanB, pc.api.settingEngine.candidates.ICELite, pc.api.mediaEngine, connectionRoleFromDtlsRole(defaultDtlsRoleOffer), candidates, iceParams, mediaSections, pc.ICEGatheringState())
}

// generateMatchedSDP generates a SDP and takes the remote state into account
// this is used everytime we have a RemoteDescription
func (pc *PeerConnection) generateMatchedSDP(useIdentity bool, includeUnmatched bool, connectionRole sdp.ConnectionRole) (*sdp.SessionDescription, error) {
	d := sdp.NewJSEPSessionDescription(useIdentity)
	if err := addFingerprints(d, pc.configuration.Certificates[0]); err != nil {
		return nil, err
	}

	iceParams, err := pc.iceGatherer.GetLocalParameters()
	if err != nil {
		return nil, err
	}

	candidates, err := pc.iceGatherer.GetLocalCandidates()
	if err != nil {
		return nil, err
	}

	var t *RTPTransceiver
	localTransceivers := append([]*RTPTransceiver{}, pc.GetTransceivers()...)
	detectedPlanB := descriptionIsPlanB(pc.RemoteDescription())
	mediaSections := []mediaSection{}

	for _, media := range pc.RemoteDescription().parsed.MediaDescriptions {
		midValue := getMidValue(media)
		if midValue == "" {
			return nil, fmt.Errorf("RemoteDescription contained media section without mid value")
		}

		if media.MediaName.Media == "application" {
			mediaSections = append(mediaSections, mediaSection{id: midValue, data: true})
			continue
		}

		kind := NewRTPCodecType(media.MediaName.Media)
		direction := getPeerDirection(media)
		if kind == 0 || direction == RTPTransceiverDirection(Unknown) {
			continue
		}

		t, localTransceivers = satisfyTypeAndDirection(kind, direction, localTransceivers)
		mediaTransceivers := []*RTPTransceiver{t}
		switch pc.configuration.SDPSemantics {
		case SDPSemanticsUnifiedPlanWithFallback:
			// If no match, process as unified-plan
			if !detectedPlanB {
				break
			}
			// If there was a match, fall through to plan-b
			fallthrough
		case SDPSemanticsPlanB:
			if !detectedPlanB {
				return nil, &rtcerr.TypeError{Err: ErrIncorrectSDPSemantics}
			}
			// If we're responding to a plan-b offer, then we should try to fill up this
			// media entry with all matching local transceivers
			for {
				// keep going until we can't get any more
				t, localTransceivers = satisfyTypeAndDirection(kind, direction, localTransceivers)
				if t.Direction() == RTPTransceiverDirectionInactive {
					break
				}
				mediaTransceivers = append(mediaTransceivers, t)
			}
		case SDPSemanticsUnifiedPlan:
			if detectedPlanB {
				return nil, &rtcerr.TypeError{Err: ErrIncorrectSDPSemantics}
			}
		}

		mediaSections = append(mediaSections, mediaSection{id: midValue, transceivers: mediaTransceivers})
	}

	// If we are offering also include unmatched local transceivers
	if !detectedPlanB && includeUnmatched {
		for _, t := range localTransceivers {
			mediaSections = append(mediaSections, mediaSection{id: strconv.Itoa(len(mediaSections)), transceivers: []*RTPTransceiver{t}})
		}
	}

	if pc.configuration.SDPSemantics == SDPSemanticsUnifiedPlanWithFallback && detectedPlanB {
		pc.log.Info("Plan-B Offer detected; responding with Plan-B Answer")
	}

	return populateSDP(d, detectedPlanB, pc.api.settingEngine.candidates.ICELite, pc.api.mediaEngine, connectionRole, candidates, iceParams, mediaSections, pc.ICEGatheringState())
}
