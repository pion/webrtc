// +build js

// Package webrtc implements the WebRTC 1.0 as defined in W3C WebRTC specification document.
package webrtc

import (
	"syscall/js"

	"github.com/pions/webrtc/pkg/rtcerr"
)

// PeerConnection represents a WebRTC connection that establishes a
// peer-to-peer communications with another PeerConnection instance in a
// browser, or to another endpoint implementing the required protocols.
type PeerConnection struct {
	// Pointer to the underlying JavaScript RTCPeerConnection object.
	underlying js.Value

	// Keep track of handlers/callbacks so we can call Release as required by the
	// syscall/js API. Initially nil.
	onSignalingStateChangeHandler    *js.Func
	onDataChannelHandler             *js.Func
	onICEConectionStateChangeHandler *js.Func
	onICECandidateHandler            *js.Func
	onNegotiationNeededHandler       *js.Func
}

// NewPeerConnection creates a peerconnection with the default
// codecs.
func NewPeerConnection(configuration Configuration) (_ *PeerConnection, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = recoveryToError(e)
		}
	}()
	configMap := configurationToValue(configuration)
	underlying := js.Global().Get("window").Get("RTCPeerConnection").New(configMap)
	return &PeerConnection{
		underlying: underlying,
	}, nil
}

// OnSignalingStateChange sets an event handler which is invoked when the
// peer connection's signaling state changes
func (pc *PeerConnection) OnSignalingStateChange(f func(SignalingState)) {
	if pc.onSignalingStateChangeHandler != nil {
		oldHandler := pc.onSignalingStateChangeHandler
		defer oldHandler.Release()
	}
	onSignalingStateChangeHandler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		// TODO(albrow): Protect args access; recover from panics.
		state := newSignalingState(args[0].String())
		go f(state)
		return js.Undefined()
	})
	pc.onSignalingStateChangeHandler = &onSignalingStateChangeHandler
	pc.underlying.Set("onsignalingstatechange", onSignalingStateChangeHandler)
}

// OnDataChannel sets an event handler which is invoked when a data
// channel message arrives from a remote peer.
func (pc *PeerConnection) OnDataChannel(f func(*DataChannel)) {
	if pc.onDataChannelHandler != nil {
		oldHandler := pc.onDataChannelHandler
		defer oldHandler.Release()
	}
	onDataChannelHandler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		// BUG(albrow): This reference to the underlying DataChannel doesn't know
		// about any other references to the same DataChannel. This might result in
		// memory leaks where we don't clean up handler functions. Could possibly
		// fix by keeping a mutex-protected list of all DataChannel references as a
		// property of this PeerConnection, but at the cost of additional overhead.
		dataChannel := &DataChannel{
			underlying: args[0].Get("channel"),
		}
		go f(dataChannel)
		return js.Undefined()
	})
	pc.onDataChannelHandler = &onDataChannelHandler
	pc.underlying.Set("ondatachannel", onDataChannelHandler)
}

// OnTrack sets an event handler which is called when remote track
// arrives from a remote peer.
// func (pc *PeerConnection) OnTrack(f func(*Track, *RTPReceiver)) {
// }

// OnICEConnectionStateChange sets an event handler which is called
// when an ICE connection state is changed.
func (pc *PeerConnection) OnICEConnectionStateChange(f func(ICEConnectionState)) {
	if pc.onICEConectionStateChangeHandler != nil {
		oldHandler := pc.onICEConectionStateChangeHandler
		defer oldHandler.Release()
	}
	onICEConectionStateChangeHandler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		connectionState := newICEConnectionState(pc.underlying.Get("iceConnectionState").String())
		go f(connectionState)
		return js.Undefined()
	})
	pc.onICEConectionStateChangeHandler = &onICEConectionStateChangeHandler
	pc.underlying.Set("oniceconnectionstatechange", onICEConectionStateChangeHandler)
}

func (pc *PeerConnection) checkConfiguration(configuration Configuration) error {
	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-setconfiguration (step #2)
	if pc.ConnectionState() == PeerConnectionStateClosed {
		return &rtcerr.InvalidStateError{Err: ErrConnectionClosed}
	}

	existingConfig := pc.GetConfiguration()
	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #3)
	if configuration.PeerIdentity != "" {
		if configuration.PeerIdentity != existingConfig.PeerIdentity {
			return &rtcerr.InvalidModificationError{Err: ErrModifyingPeerIdentity}
		}
	}

	// TODO: Enable these checks once Certificates are supported.
	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #4)
	// if len(configuration.Certificates) > 0 {
	// 	if len(configuration.Certificates) != len(existingConfiguration.Certificates) {
	// 		return &rtcerr.InvalidModificationError{Err: ErrModifyingCertificates}
	// 	}

	// 	for i, certificate := range configuration.Certificates {
	// 		if !pc.configuration.Certificates[i].Equals(certificate) {
	// 			return &rtcerr.InvalidModificationError{Err: ErrModifyingCertificates}
	// 		}
	// 	}
	// 	pc.configuration.Certificates = configuration.Certificates
	// }

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #5)
	if configuration.BundlePolicy != BundlePolicy(Unknown) {
		if configuration.BundlePolicy != existingConfig.BundlePolicy {
			return &rtcerr.InvalidModificationError{Err: ErrModifyingBundlePolicy}
		}
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #6)
	if configuration.RTCPMuxPolicy != RTCPMuxPolicy(Unknown) {
		if configuration.RTCPMuxPolicy != existingConfig.RTCPMuxPolicy {
			return &rtcerr.InvalidModificationError{Err: ErrModifyingRTCPMuxPolicy}
		}
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #7)
	if configuration.ICECandidatePoolSize != 0 {
		if configuration.ICECandidatePoolSize != existingConfig.ICECandidatePoolSize &&
			pc.LocalDescription() != nil {
			return &rtcerr.InvalidModificationError{Err: ErrModifyingICECandidatePoolSize}
		}
	}

	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #11)
	if len(configuration.ICEServers) > 0 {
		// https://www.w3.org/TR/webrtc/#set-the-configuration (step #11.3)
		for _, server := range configuration.ICEServers {
			if _, err := server.validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetConfiguration updates the configuration of this PeerConnection object.
func (pc *PeerConnection) SetConfiguration(configuration Configuration) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = recoveryToError(e)
		}
	}()
	if err := pc.checkConfiguration(configuration); err != nil {
		return err
	}
	configMap := configurationToValue(configuration)
	pc.underlying.Call("setConfiguration", configMap)
	return nil
}

// GetConfiguration returns a Configuration object representing the current
// configuration of this PeerConnection object. The returned object is a
// copy and direct mutation on it will not take affect until SetConfiguration
// has been called with Configuration passed as its only argument.
// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-getconfiguration
func (pc *PeerConnection) GetConfiguration() Configuration {
	return valueToConfiguration(pc.underlying.Call("getConfiguration"))
}

// CreateOffer starts the PeerConnection and generates the localDescription
func (pc *PeerConnection) CreateOffer(options *OfferOptions) (_ SessionDescription, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = recoveryToError(e)
		}
	}()
	promise := pc.underlying.Call("createOffer", offerOptionsToValue(options))
	desc, err := awaitPromise(promise)
	if err != nil {
		return SessionDescription{}, err
	}
	return *valueToSessionDescription(desc), nil
}

// CreateAnswer starts the PeerConnection and generates the localDescription
func (pc *PeerConnection) CreateAnswer(options *AnswerOptions) (_ SessionDescription, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = recoveryToError(e)
		}
	}()
	promise := pc.underlying.Call("createAnswer", answerOptionsToValue(options))
	desc, err := awaitPromise(promise)
	if err != nil {
		return SessionDescription{}, err
	}
	return *valueToSessionDescription(desc), nil
}

// SetLocalDescription sets the SessionDescription of the local peer
func (pc *PeerConnection) SetLocalDescription(desc SessionDescription) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = recoveryToError(e)
		}
	}()
	promise := pc.underlying.Call("setLocalDescription", sessionDescriptionToValue(&desc))
	_, err = awaitPromise(promise)
	return err
}

// LocalDescription returns PendingLocalDescription if it is not null and
// otherwise it returns CurrentLocalDescription. This property is used to
// determine if setLocalDescription has already been called.
// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-localdescription
func (pc *PeerConnection) LocalDescription() *SessionDescription {
	return valueToSessionDescription(pc.underlying.Get("localDescription"))
}

// SetRemoteDescription sets the SessionDescription of the remote peer
func (pc *PeerConnection) SetRemoteDescription(desc SessionDescription) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = recoveryToError(e)
		}
	}()
	promise := pc.underlying.Call("setRemoteDescription", sessionDescriptionToValue(&desc))
	_, err = awaitPromise(promise)
	return err
}

// RemoteDescription returns PendingRemoteDescription if it is not null and
// otherwise it returns CurrentRemoteDescription. This property is used to
// determine if setRemoteDescription has already been called.
// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-remotedescription
func (pc *PeerConnection) RemoteDescription() *SessionDescription {
	return valueToSessionDescription(pc.underlying.Get("remoteDescription"))
}

// AddICECandidate accepts an ICE candidate string and adds it
// to the existing set of candidates
func (pc *PeerConnection) AddICECandidate(candidate ICECandidateInit) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = recoveryToError(e)
		}
	}()
	promise := pc.underlying.Call("addIceCandidate", iceCandidateInitToValue(candidate))
	_, err = awaitPromise(promise)
	return err
}

// ICEConnectionState returns the ICE connection state of the
// PeerConnection instance.
func (pc *PeerConnection) ICEConnectionState() ICEConnectionState {
	return newICEConnectionState(pc.underlying.Get("iceConnectionState").String())
}

// TODO(albrow): This function doesn't exist in the Go implementation.
// TODO(albrow): Follow the spec more closely. Handler should accept
// RTCPeerConnectionIceEvent instead of *string.
// https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/onicecandidate
func (pc *PeerConnection) OnICECandidate(f func(candidate *string)) {
	if pc.onICECandidateHandler != nil {
		oldHandler := pc.onICECandidateHandler
		defer oldHandler.Release()
	}
	onICECandidateHandler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		// TODO(albrow): Protect args access; recover from panics.
		candidate := valueToStringPointer(args[0].Get("candidate"))
		go f(candidate)
		return js.Undefined()
	})
	pc.onICECandidateHandler = &onICECandidateHandler
	pc.underlying.Set("onicecandidate", onICECandidateHandler)
}

// TODO(albrow): This function doesn't exist in the Go implementation.
func (pc *PeerConnection) OnNegotiationNeeded(f func()) {
	if pc.onNegotiationNeededHandler != nil {
		oldHandler := pc.onNegotiationNeededHandler
		defer oldHandler.Release()
	}
	onNegotiationNeededHandler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go f()
		return js.Undefined()
	})
	pc.onNegotiationNeededHandler = &onNegotiationNeededHandler
	pc.underlying.Set("onnegotiationneeded", onNegotiationNeededHandler)
}

// // GetSenders returns the RTPSender that are currently attached to this PeerConnection
// func (pc *PeerConnection) GetSenders() []*RTPSender {
// }

// // GetReceivers returns the RTPReceivers that are currently attached to this RTCPeerConnection
// func (pc *PeerConnection) GetReceivers() []*RTPReceiver {
// }

// // GetTransceivers returns the RTCRtpTransceiver that are currently attached to this RTCPeerConnection
// func (pc *PeerConnection) GetTransceivers() []*RTPTransceiver {
// }

// // AddTrack adds a Track to the PeerConnection
// func (pc *PeerConnection) AddTrack(track *Track) (*RTPSender, error) {
// }

// func (pc *PeerConnection) RemoveTrack() {
// }

// func (pc *PeerConnection) AddTransceiver() RTPTransceiver {
// }

// CreateDataChannel creates a new DataChannel object with the given label
// and optional DataChannelInit used to configure properties of the
// underlying channel such as data reliability.
func (pc *PeerConnection) CreateDataChannel(label string, options *DataChannelInit) (_ *DataChannel, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = recoveryToError(e)
		}
	}()
	channel := pc.underlying.Call("createDataChannel", label, dataChannelInitToValue(options))
	return &DataChannel{
		underlying: channel,
	}, nil
}

// SetIdentityProvider is used to configure an identity provider to generate identity assertions
func (pc *PeerConnection) SetIdentityProvider(provider string) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = recoveryToError(e)
		}
	}()
	pc.underlying.Call("setIdentityProvider", provider)
	return nil
}

// Note(albrow) SendRTCP is not included in MDN WebRTC documentation.
// SendRTCP sends a user provided RTCP packet to the connected peer
// If no peer is connected the packet is discarded
// func (pc *PeerConnection) SendRTCP(pkt rtcp.Packet) error {
// 	return errors.New("Not yet implemented")
// }

// Close ends the PeerConnection
func (pc *PeerConnection) Close() (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = recoveryToError(e)
		}
	}()

	pc.underlying.Call("close")

	// Release any handlers as required by the syscall/js API.
	if pc.onSignalingStateChangeHandler != nil {
		pc.onSignalingStateChangeHandler.Release()
	}
	if pc.onDataChannelHandler != nil {
		pc.onDataChannelHandler.Release()
	}
	if pc.onICEConectionStateChangeHandler != nil {
		pc.onICEConectionStateChangeHandler.Release()
	}
	if pc.onICECandidateHandler != nil {
		pc.onICECandidateHandler.Release()
	}
	if pc.onNegotiationNeededHandler != nil {
		pc.onNegotiationNeededHandler.Release()
	}

	return nil
}

// NewTrack Creates a new Track
// func (pc *PeerConnection) NewTrack(payloadType uint8, ssrc uint32, id, label string) (*Track, error) {
// 	return nil, errors.New("Not yet implemented")
// }

// CurrentLocalDescription represents the local description that was
// successfully negotiated the last time the PeerConnection transitioned
// into the stable state plus any local candidates that have been generated
// by the ICEAgent since the offer or answer was created.
func (pc *PeerConnection) CurrentLocalDescription() *SessionDescription {
	desc := pc.underlying.Get("currentLocalDescription")
	return valueToSessionDescription(desc)
}

// PendingLocalDescription represents a local description that is in the
// process of being negotiated plus any local candidates that have been
// generated by the ICEAgent since the offer or answer was created. If the
// PeerConnection is in the stable state, the value is null.
func (pc *PeerConnection) PendingLocalDescription() *SessionDescription {
	desc := pc.underlying.Get("pendingLocalDescription")
	return valueToSessionDescription(desc)
}

// CurrentRemoteDescription represents the last remote description that was
// successfully negotiated the last time the PeerConnection transitioned
// into the stable state plus any remote candidates that have been supplied
// via AddICECandidate() since the offer or answer was created.
func (pc *PeerConnection) CurrentRemoteDescription() *SessionDescription {
	desc := pc.underlying.Get("currentRemoteDescription")
	return valueToSessionDescription(desc)
}

// PendingRemoteDescription represents a remote description that is in the
// process of being negotiated, complete with any remote candidates that
// have been supplied via AddICECandidate() since the offer or answer was
// created. If the PeerConnection is in the stable state, the value is
// null.
func (pc *PeerConnection) PendingRemoteDescription() *SessionDescription {
	desc := pc.underlying.Get("pendingRemoteDescription")
	return valueToSessionDescription(desc)
}

// SignalingState returns the signaling state of the PeerConnection instance.
func (pc *PeerConnection) SignalingState() SignalingState {
	rawState := pc.underlying.Get("signalingState").String()
	return newSignalingState(rawState)
}

// ICEGatheringState attribute the ICE gathering state of the PeerConnection
// instance.
func (pc *PeerConnection) ICEGatheringState() ICEGatheringState {
	rawState := pc.underlying.Get("iceGatheringState").String()
	return newICEGatheringState(rawState)
}

// ConnectionState attribute the connection state of the PeerConnection
// instance.
func (pc *PeerConnection) ConnectionState() PeerConnectionState {
	rawState := pc.underlying.Get("connectionState").String()
	return newPeerConnectionState(rawState)
}

// Converts a Configuration to js.Value so it can be passed
// through to the JavaScript WebRTC API. Any zero values are converted to
// js.Undefined(), which will result in the default value being used.
func configurationToValue(configuration Configuration) js.Value {
	return js.ValueOf(map[string]interface{}{
		"iceServers":           iceServersToValue(configuration.ICEServers),
		"iceTransportPolicy":   stringEnumToValueOrUndefined(configuration.ICETransportPolicy.String()),
		"bundlePolicy":         stringEnumToValueOrUndefined(configuration.BundlePolicy.String()),
		"rtcpMuxPolicy":        stringEnumToValueOrUndefined(configuration.RTCPMuxPolicy.String()),
		"peerIdentity":         stringToValueOrUndefined(configuration.PeerIdentity),
		"iceCandidatePoolSize": uint8ToValueOrUndefined(configuration.ICECandidatePoolSize),

		// TODO(albrow): Docs for RTCCertificate are underspecified.
		// https://developer.mozilla.org/en-US/docs/Web/API/RTCCertificate How
		// should we handle this?
		// "certificates": configuration.Certificates,
	})
}

func iceServersToValue(iceServers []ICEServer) js.Value {
	if len(iceServers) == 0 {
		return js.Undefined()
	}
	maps := make([]interface{}, len(iceServers))
	for i, server := range iceServers {
		maps[i] = iceServerToValue(server)
	}
	return js.ValueOf(maps)
}

func iceServerToValue(server ICEServer) js.Value {
	return js.ValueOf(map[string]interface{}{
		"urls":     stringsToValue(server.URLs), // required
		"username": stringToValueOrUndefined(server.Username),
		// TODO(albrow): credential is not currently supported.
		// "credential":     interfaceToValueOrUndefined(server.Credential),
		"credentialType": stringEnumToValueOrUndefined(server.CredentialType.String()),
	})
}

func valueToConfiguration(configValue js.Value) Configuration {
	if configValue == js.Null() || configValue == js.Undefined() {
		return Configuration{}
	}
	return Configuration{
		ICEServers:           valueToICEServers(configValue.Get("iceServers")),
		ICETransportPolicy:   newICETransportPolicy(valueToStringOrZero(configValue.Get("iceTransportPolicy"))),
		BundlePolicy:         newBundlePolicy(valueToStringOrZero(configValue.Get("bundlePolicy"))),
		RTCPMuxPolicy:        newRTCPMuxPolicy(valueToStringOrZero(configValue.Get("rtcpMuxPolicy"))),
		PeerIdentity:         valueToStringOrZero(configValue.Get("peerIdentity")),
		ICECandidatePoolSize: valueToUint8OrZero(configValue.Get("iceCandidatePoolSize")),

		// TODO(albrow): Docs for RTCCertificate are underspecified.
		// https://developer.mozilla.org/en-US/docs/Web/API/RTCCertificate How
		// should we handle this?
		// Certificates []Certificate
	}
}

func valueToICEServers(iceServersValue js.Value) []ICEServer {
	if iceServersValue == js.Null() || iceServersValue == js.Undefined() {
		return nil
	}
	iceServers := make([]ICEServer, iceServersValue.Length())
	for i := 0; i < iceServersValue.Length(); i++ {
		iceServers[i] = valueToICEServer(iceServersValue.Index(i))
	}
	return iceServers
}

func valueToICEServer(iceServerValue js.Value) ICEServer {
	return ICEServer{
		URLs:     valueToStrings(iceServerValue.Get("urls")), // required
		Username: valueToStringOrZero(iceServerValue.Get("username")),
		// TODO(albrow): Handle Credential which might have different types.
		// Credential: iceServerValue.Get("credential"),
		CredentialType: newICECredentialType(valueToStringOrZero(iceServerValue.Get("credentialType"))),
	}
}

func sessionDescriptionToValue(desc *SessionDescription) js.Value {
	if desc == nil {
		return js.Undefined()
	}
	return js.ValueOf(map[string]interface{}{
		"type": desc.Type.String(),
		"sdp":  desc.SDP,
	})
}

func valueToSessionDescription(descValue js.Value) *SessionDescription {
	if descValue == js.Null() || descValue == js.Undefined() {
		return nil
	}
	return &SessionDescription{
		Type: newSDPType(descValue.Get("type").String()),
		SDP:  descValue.Get("sdp").String(),
	}
}

func offerOptionsToValue(offerOptions *OfferOptions) js.Value {
	if offerOptions == nil {
		return js.Undefined()
	}
	return js.ValueOf(map[string]interface{}{
		"iceRestart":             offerOptions.ICERestart,
		"voiceActivityDetection": offerOptions.VoiceActivityDetection,
	})
}

func answerOptionsToValue(answerOptions *AnswerOptions) js.Value {
	if answerOptions == nil {
		return js.Undefined()
	}
	return js.ValueOf(map[string]interface{}{
		"voiceActivityDetection": answerOptions.VoiceActivityDetection,
	})
}

func iceCandidateInitToValue(candidate ICECandidateInit) js.Value {
	return js.ValueOf(map[string]interface{}{
		"candidate":        candidate.Candidate,
		"sdpMid":           stringPointerToValue(candidate.SDPMid),
		"sdpMLineIndex":    uint16PointerToValue(candidate.SDPMLineIndex),
		"usernameFragment": candidate.UsernameFragment,
	})
}

func dataChannelInitToValue(options *DataChannelInit) js.Value {
	if options == nil {
		return js.Undefined()
	}

	maxPacketLifeTime := uint16PointerToValue(options.MaxPacketLifeTime)
	return js.ValueOf(map[string]interface{}{
		"ordered":           boolPointerToValue(options.Ordered),
		"maxPacketLifeTime": maxPacketLifeTime,
		// See https://bugs.chromium.org/p/chromium/issues/detail?id=696681
		// Chrome calls this "maxRetransmitTime"
		"maxRetransmitTime": maxPacketLifeTime,
		"maxRetransmits":    uint16PointerToValue(options.MaxRetransmits),
		"protocol":          stringPointerToValue(options.Protocol),
		"negotiated":        boolPointerToValue(options.Negotiated),
		"id":                uint16PointerToValue(options.ID),

		// Note(albrow) Priority is not included in MDN WebRTC documentation. Should
		// we include it here?
		// "priority": options.Priority
	})
}
