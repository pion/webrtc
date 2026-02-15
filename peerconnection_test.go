// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/sdp/v3"
	"github.com/pion/transport/v4/test"
	"github.com/pion/webrtc/v4/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
)

// newPair creates two new peer connections (an offerer and an answerer)
// *without* using an api (i.e. using the default settings).
func newPair() (pcOffer *PeerConnection, pcAnswer *PeerConnection, err error) {
	pca, err := NewPeerConnection(Configuration{})
	if err != nil {
		return nil, nil, err
	}

	pcb, err := NewPeerConnection(Configuration{})
	if err != nil {
		return nil, nil, err
	}

	return pca, pcb, nil
}

type signalPairOptions struct {
	disableInitialDataChannel bool
	modificationFunc          func(string) string
}

func withModificationFunc(f func(string) string) func(*signalPairOptions) {
	return func(o *signalPairOptions) {
		o.modificationFunc = f
	}
}

func withDisableInitialDataChannel(disable bool) func(*signalPairOptions) {
	return func(o *signalPairOptions) {
		o.disableInitialDataChannel = disable
	}
}

func signalPairWithOptions(
	pcOffer *PeerConnection,
	pcAnswer *PeerConnection,
	opts ...func(*signalPairOptions),
) error {
	var options signalPairOptions
	for _, o := range opts {
		o(&options)
	}

	modificationFunc := options.modificationFunc
	if modificationFunc == nil {
		modificationFunc = func(s string) string { return s }
	}

	if !options.disableInitialDataChannel {
		// Note(albrow): We need to create a data channel in order to trigger ICE
		// candidate gathering in the background for the JavaScript/Wasm bindings. If
		// we don't do this, the complete offer including ICE candidates will never be
		// generated.
		if _, err := pcOffer.CreateDataChannel("initial_data_channel", nil); err != nil {
			return err
		}
	}

	offer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		return err
	}
	offerGatheringComplete := GatheringCompletePromise(pcOffer)
	if err = pcOffer.SetLocalDescription(offer); err != nil {
		return err
	}
	<-offerGatheringComplete

	offer.SDP = modificationFunc(pcOffer.LocalDescription().SDP)
	if err = pcAnswer.SetRemoteDescription(offer); err != nil {
		return err
	}

	answer, err := pcAnswer.CreateAnswer(nil)
	if err != nil {
		return err
	}
	answerGatheringComplete := GatheringCompletePromise(pcAnswer)
	if err = pcAnswer.SetLocalDescription(answer); err != nil {
		return err
	}
	<-answerGatheringComplete

	return pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription())
}

func signalPairWithModification(
	pcOffer *PeerConnection,
	pcAnswer *PeerConnection,
	modificationFunc func(string) string,
) error {
	return signalPairWithOptions(
		pcOffer,
		pcAnswer,
		withModificationFunc(modificationFunc),
	)
}

func signalPair(pcOffer *PeerConnection, pcAnswer *PeerConnection) error {
	return signalPairWithModification(
		pcOffer,
		pcAnswer,
		func(sessionDescription string) string { return sessionDescription },
	)
}

func offerMediaHasDirection(offer SessionDescription, kind RTPCodecType, direction RTPTransceiverDirection) bool {
	parsed := &sdp.SessionDescription{}
	if err := parsed.Unmarshal([]byte(offer.SDP)); err != nil {
		return false
	}

	for _, media := range parsed.MediaDescriptions {
		if media.MediaName.Media == kind.String() {
			_, exists := media.Attribute(direction.String())

			return exists
		}
	}

	return false
}

func untilConnectionState(state PeerConnectionState, peers ...*PeerConnection) *sync.WaitGroup {
	var triggered sync.WaitGroup
	triggered.Add(len(peers))

	for _, p := range peers {
		var done atomic.Value
		done.Store(false)
		hdlr := func(p PeerConnectionState) {
			if val, ok := done.Load().(bool); ok && (!val && p == state) {
				done.Store(true)
				triggered.Done()
			}
		}

		p.OnConnectionStateChange(hdlr)
	}

	return &triggered
}

func TestNew(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{
		ICEServers: []ICEServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
				},
				Username: "unittest",
			},
		},
		ICETransportPolicy:   ICETransportPolicyRelay,
		BundlePolicy:         BundlePolicyMaxCompat,
		RTCPMuxPolicy:        RTCPMuxPolicyNegotiate,
		PeerIdentity:         "unittest",
		ICECandidatePoolSize: 1,
	})
	assert.NoError(t, err)
	assert.NotNil(t, pc)
	assert.NoError(t, pc.Close())
}

func TestPeerConnection_SetConfiguration(t *testing.T) {
	// Note: These tests don't include ICEServer.Credential,
	// ICEServer.CredentialType, or Certificates because those are not supported
	// in the WASM bindings.

	for _, test := range []struct {
		name    string
		init    func() (*PeerConnection, error)
		config  Configuration
		wantErr error
	}{
		{
			name: "valid",
			init: func() (*PeerConnection, error) {
				pc, err := NewPeerConnection(Configuration{
					ICECandidatePoolSize: 1,
				})
				if err != nil {
					return pc, err
				}

				err = pc.SetConfiguration(Configuration{
					ICEServers: []ICEServer{
						{
							URLs: []string{
								"stun:stun.l.google.com:19302",
							},
							Username: "unittest",
						},
					},
					ICETransportPolicy:          ICETransportPolicyAll,
					BundlePolicy:                BundlePolicyBalanced,
					RTCPMuxPolicy:               RTCPMuxPolicyRequire,
					ICECandidatePoolSize:        1,
					AlwaysNegotiateDataChannels: true,
					RTPHeaderEncryptionPolicy:   RTPHeaderEncryptionPolicyNegotiate,
				})
				if err != nil {
					return pc, err
				}

				return pc, nil
			},
			config:  Configuration{},
			wantErr: nil,
		},
		{
			name: "closed connection",
			init: func() (*PeerConnection, error) {
				pc, err := NewPeerConnection(Configuration{})
				assert.Nil(t, err)

				err = pc.Close()
				assert.Nil(t, err)

				return pc, err
			},
			config:  Configuration{},
			wantErr: &rtcerr.InvalidStateError{Err: ErrConnectionClosed},
		},
		{
			name: "update PeerIdentity",
			init: func() (*PeerConnection, error) {
				return NewPeerConnection(Configuration{})
			},
			config: Configuration{
				PeerIdentity: "unittest",
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingPeerIdentity},
		},
		{
			name: "update BundlePolicy",
			init: func() (*PeerConnection, error) {
				return NewPeerConnection(Configuration{})
			},
			config: Configuration{
				BundlePolicy: BundlePolicyMaxCompat,
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingBundlePolicy},
		},
		{
			name: "update RTCPMuxPolicy",
			init: func() (*PeerConnection, error) {
				return NewPeerConnection(Configuration{})
			},
			config: Configuration{
				RTCPMuxPolicy: RTCPMuxPolicyNegotiate,
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingRTCPMuxPolicy},
		},
		{
			name: "update ICECandidatePoolSize",
			init: func() (*PeerConnection, error) {
				pc, err := NewPeerConnection(Configuration{
					ICECandidatePoolSize: 0,
				})
				if err != nil {
					return pc, err
				}
				offer, err := pc.CreateOffer(nil)
				if err != nil {
					return pc, err
				}
				err = pc.SetLocalDescription(offer)
				if err != nil {
					return pc, err
				}

				return pc, nil
			},
			config: Configuration{
				ICECandidatePoolSize: 1,
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingICECandidatePoolSize},
		},
		{
			name: "enable AlwaysNegotiateDataChannels",
			init: func() (*PeerConnection, error) {
				return NewPeerConnection(Configuration{})
			},
			config:  Configuration{AlwaysNegotiateDataChannels: true},
			wantErr: nil,
		},
	} {
		pc, err := test.init()
		assert.NoError(t, err, "SetConfiguration %q: init failed", test.name)

		err = pc.SetConfiguration(test.config)
		// We use Equal instead of ErrorIs because the error is a pointer to a struct.
		assert.Equal(t, test.wantErr, err, "SetConfiguration %q", test.name)

		assert.NoError(t, pc.Close())
	}
}

func TestPeerConnection_GetConfiguration(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	expected := Configuration{
		ICEServers:                  []ICEServer{},
		ICETransportPolicy:          ICETransportPolicyAll,
		BundlePolicy:                BundlePolicyBalanced,
		RTCPMuxPolicy:               RTCPMuxPolicyRequire,
		ICECandidatePoolSize:        0,
		AlwaysNegotiateDataChannels: false,
		RTPHeaderEncryptionPolicy:   RTPHeaderEncryptionPolicyNegotiate,
	}
	actual := pc.GetConfiguration()
	assert.True(t, &expected != &actual)
	assert.Equal(t, expected.ICEServers, actual.ICEServers)
	assert.Equal(t, expected.ICETransportPolicy, actual.ICETransportPolicy)
	assert.Equal(t, expected.BundlePolicy, actual.BundlePolicy)
	assert.Equal(t, expected.RTCPMuxPolicy, actual.RTCPMuxPolicy)
	// nolint:godox
	// TODO(albrow): Uncomment this after #513 is fixed.
	// See: https://github.com/pion/webrtc/issues/513.
	// assert.Equal(t, len(expected.Certificates), len(actual.Certificates))
	assert.Equal(t, expected.ICECandidatePoolSize, actual.ICECandidatePoolSize)
	assert.Equal(t, expected.AlwaysNegotiateDataChannels, actual.AlwaysNegotiateDataChannels)
	assert.NoError(t, pc.Close())
}

const minimalOffer = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE data
a=msid-semantic: WMS
m=application 47299 DTLS/SCTP 5000
c=IN IP4 192.168.20.129
a=candidate:1966762134 1 udp 2122260223 192.168.20.129 47299 typ host generation 0
a=candidate:1966762134 1 udp 2122262783 2001:db8::1 47199 typ host generation 0
a=candidate:211962667 1 udp 2122194687 10.0.3.1 40864 typ host generation 0
a=candidate:1002017894 1 tcp 1518280447 192.168.20.129 0 typ host tcptype active generation 0
a=candidate:1109506011 1 tcp 1518214911 10.0.3.1 0 typ host tcptype active generation 0
a=ice-ufrag:1/MvHwjAyVf27aLu
a=ice-pwd:3dBU7cFOBl120v33cynDvN1E
a=ice-options:google-ice
a=fingerprint:sha-256 75:74:5A:A6:A4:E5:52:F4:A7:67:4C:01:C7:EE:91:3F:21:3D:A2:E3:53:7B:6F:30:86:F2:30:AA:65:FB:04:24
a=setup:actpass
a=mid:data
a=sctpmap:5000 webrtc-datachannel 1024
`

func TestSetRemoteDescription(t *testing.T) {
	testCases := []struct {
		desc        SessionDescription
		expectError bool
	}{
		{SessionDescription{Type: SDPTypeOffer, SDP: minimalOffer}, false},
		{SessionDescription{Type: 0, SDP: ""}, true},
	}

	for i, testCase := range testCases {
		peerConn, err := NewPeerConnection(Configuration{})
		assert.NoErrorf(t, err, "Case %d: got errror", i)

		if testCase.expectError {
			assert.Error(t, peerConn.SetRemoteDescription(testCase.desc))
		} else {
			assert.NoError(t, peerConn.SetRemoteDescription(testCase.desc))
		}

		assert.NoError(t, peerConn.Close())
	}
}

func TestCreateOfferAnswer(t *testing.T) {
	offerPeerConn, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	answerPeerConn, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = offerPeerConn.CreateDataChannel("test-channel", nil)
	assert.NoError(t, err)

	offer, err := offerPeerConn.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, offerPeerConn.SetLocalDescription(offer))

	assert.NoError(t, answerPeerConn.SetRemoteDescription(offer))

	answer, err := answerPeerConn.CreateAnswer(nil)
	assert.NoError(t, err)

	assert.NoError(t, answerPeerConn.SetLocalDescription(answer))
	assert.NoError(t, offerPeerConn.SetRemoteDescription(answer))

	// after setLocalDescription(answer), signaling state should be stable.
	// so CreateAnswer should return an InvalidStateError
	assert.Equal(t, answerPeerConn.SignalingState(), SignalingStateStable)
	_, err = answerPeerConn.CreateAnswer(nil)
	assert.Error(t, err)

	closePairNow(t, offerPeerConn, answerPeerConn)
}

func TestPeerConnection_EventHandlers(t *testing.T) {
	pcOffer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	pcAnswer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	// wasCalled is a list of event handlers that were called.
	wasCalled := []string{}
	wasCalledMut := &sync.Mutex{}
	// wg is used to wait for all event handlers to be called.
	wg := &sync.WaitGroup{}
	wg.Add(6)

	// Each sync.Once is used to ensure that we call wg.Done once for each event
	// handler and don't add multiple entries to wasCalled. The event handlers can
	// be called more than once in some cases.
	onceOffererOnICEConnectionStateChange := &sync.Once{}
	onceOffererOnConnectionStateChange := &sync.Once{}
	onceOffererOnSignalingStateChange := &sync.Once{}
	onceAnswererOnICEConnectionStateChange := &sync.Once{}
	onceAnswererOnConnectionStateChange := &sync.Once{}
	onceAnswererOnSignalingStateChange := &sync.Once{}

	// Register all the event handlers.
	pcOffer.OnICEConnectionStateChange(func(ICEConnectionState) {
		onceOffererOnICEConnectionStateChange.Do(func() {
			wasCalledMut.Lock()
			defer wasCalledMut.Unlock()
			wasCalled = append(wasCalled, "offerer OnICEConnectionStateChange")
			wg.Done()
		})
	})
	pcOffer.OnConnectionStateChange(func(PeerConnectionState) {
		onceOffererOnConnectionStateChange.Do(func() {
			wasCalledMut.Lock()
			defer wasCalledMut.Unlock()
			wasCalled = append(wasCalled, "offerer OnConnectionStateChange")
			wg.Done()
		})
	})
	pcOffer.OnSignalingStateChange(func(SignalingState) {
		onceOffererOnSignalingStateChange.Do(func() {
			wasCalledMut.Lock()
			defer wasCalledMut.Unlock()
			wasCalled = append(wasCalled, "offerer OnSignalingStateChange")
			wg.Done()
		})
	})
	pcAnswer.OnICEConnectionStateChange(func(ICEConnectionState) {
		onceAnswererOnICEConnectionStateChange.Do(func() {
			wasCalledMut.Lock()
			defer wasCalledMut.Unlock()
			wasCalled = append(wasCalled, "answerer OnICEConnectionStateChange")
			wg.Done()
		})
	})
	pcAnswer.OnConnectionStateChange(func(PeerConnectionState) {
		onceAnswererOnConnectionStateChange.Do(func() {
			wasCalledMut.Lock()
			defer wasCalledMut.Unlock()
			wasCalled = append(wasCalled, "answerer OnConnectionStateChange")
			wg.Done()
		})
	})
	pcAnswer.OnSignalingStateChange(func(SignalingState) {
		onceAnswererOnSignalingStateChange.Do(func() {
			wasCalledMut.Lock()
			defer wasCalledMut.Unlock()
			wasCalled = append(wasCalled, "answerer OnSignalingStateChange")
			wg.Done()
		})
	})

	// Use signalPair to establish a connection between pcOffer and pcAnswer. This
	// process should trigger the above event handlers.
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	// Wait for all of the event handlers to be triggered.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()
	timeout := time.After(5 * time.Second)
	select {
	case <-done:
		break
	case <-timeout:
		assert.Failf(t, "timed out waitingfor one or more events handlers to be called", "%+v *were* called", wasCalled)
	}

	closePairNow(t, pcOffer, pcAnswer)
}

func TestMultipleOfferAnswer(t *testing.T) {
	firstPeerConn, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err, "New PeerConnection")

	_, err = firstPeerConn.CreateOffer(nil)
	assert.NoError(t, err, "First Offer")
	_, err = firstPeerConn.CreateOffer(nil)
	assert.NoError(t, err, "Second Offer")

	secondPeerConn, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err, "New PeerConnection")
	secondPeerConn.OnICECandidate(func(*ICECandidate) {
	})

	_, err = secondPeerConn.CreateOffer(nil)
	assert.NoError(t, err, "First Offer")
	_, err = secondPeerConn.CreateOffer(nil)
	assert.NoError(t, err, "Second Offer")

	closePairNow(t, firstPeerConn, secondPeerConn)
}

func TestNoFingerprintInFirstMediaIfSetRemoteDescription(t *testing.T) {
	const sdpNoFingerprintInFirstMedia = `v=0
o=- 143087887 1561022767 IN IP4 192.168.84.254
s=VideoRoom 404986692241682
t=0 0
a=group:BUNDLE audio
a=msid-semantic: WMS 2867270241552712
m=video 0 UDP/TLS/RTP/SAVPF 0
a=mid:video
c=IN IP4 192.168.84.254
a=inactive
m=audio 9 UDP/TLS/RTP/SAVPF 111
c=IN IP4 192.168.84.254
a=recvonly
a=mid:audio
a=rtcp-mux
a=ice-ufrag:AS/w
a=ice-pwd:9NOgoAOMALYu/LOpA1iqg/
a=ice-options:trickle
a=fingerprint:sha-256 D2:B9:31:8F:DF:24:D8:0E:ED:D2:EF:25:9E:AF:6F:B8:34:AE:53:9C:E6:F3:8F:F2:64:15:FA:E8:7F:53:2D:38
a=setup:active
a=rtpmap:111 opus/48000/2
a=candidate:1 1 udp 2013266431 192.168.84.254 46492 typ host
a=end-of-candidates
`

	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	desc := SessionDescription{
		Type: SDPTypeOffer,
		SDP:  sdpNoFingerprintInFirstMedia,
	}

	assert.NoError(t, pc.SetRemoteDescription(desc))

	assert.NoError(t, pc.Close())
}

func TestNegotiationNeeded(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)

	pc.OnNegotiationNeeded(wg.Done)
	_, err = pc.CreateDataChannel("initial_data_channel", nil)
	assert.NoError(t, err)

	wg.Wait()

	assert.NoError(t, pc.Close())
}

func TestMultipleCreateChannel(t *testing.T) {
	var wg sync.WaitGroup

	report := test.CheckRoutines(t)
	defer report()

	// Two OnDataChannel
	// One OnNegotiationNeeded
	wg.Add(3)

	pcOffer, _ := NewPeerConnection(Configuration{})
	pcAnswer, _ := NewPeerConnection(Configuration{})

	pcAnswer.OnDataChannel(func(*DataChannel) {
		wg.Done()
	})

	pcOffer.OnNegotiationNeeded(func() {
		offer, err := pcOffer.CreateOffer(nil)
		assert.NoError(t, err)

		offerGatheringComplete := GatheringCompletePromise(pcOffer)
		assert.NoError(t, pcOffer.SetLocalDescription(offer))
		<-offerGatheringComplete
		assert.NoError(t, pcAnswer.SetRemoteDescription(*pcOffer.LocalDescription()))

		answer, err := pcAnswer.CreateAnswer(nil)
		assert.NoError(t, err)

		answerGatheringComplete := GatheringCompletePromise(pcAnswer)
		assert.NoError(t, pcAnswer.SetLocalDescription(answer))
		<-answerGatheringComplete
		err = pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription())
		assert.NoError(t, err)

		wg.Done()
	})

	_, err := pcOffer.CreateDataChannel("initial_data_channel_0", nil)
	assert.NoError(t, err)

	_, err = pcOffer.CreateDataChannel("initial_data_channel_1", nil)
	assert.NoError(t, err)
	wg.Wait()

	closePairNow(t, pcOffer, pcAnswer)
}

// Assert that candidates are gathered by calling SetLocalDescription, not SetRemoteDescription.
func TestGatherOnSetLocalDescription(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOfferGathered := make(chan SessionDescription)
	pcAnswerGathered := make(chan SessionDescription)

	s := SettingEngine{}
	api := NewAPI(WithSettingEngine(s))

	pcOffer, err := api.NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	// We need to create a data channel in order to trigger ICE
	_, err = pcOffer.CreateDataChannel("initial_data_channel", nil)
	assert.NoError(t, err)

	pcOffer.OnICECandidate(func(i *ICECandidate) {
		if i == nil {
			close(pcOfferGathered)
		}
	})

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))

	<-pcOfferGathered

	pcAnswer, err := api.NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pcAnswer.OnICECandidate(func(i *ICECandidate) {
		if i == nil {
			close(pcAnswerGathered)
		}
	})

	assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

	select {
	case <-pcAnswerGathered:
		assert.Fail(t, "pcAnswer started gathering with no SetLocalDescription")
	// Gathering is async, not sure of a better way to catch this currently
	case <-time.After(3 * time.Second):
	}

	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)
	assert.NoError(t, pcAnswer.SetLocalDescription(answer))
	<-pcAnswerGathered
	closePairNow(t, pcOffer, pcAnswer)
}

// Assert that candidates are flushed by calling SetLocalDescription if ICECandidatePoolSize > 0.
func TestFlushOnSetLocalDescription(t *testing.T) {
	if runtime.GOARCH == "wasm" {
		t.Skip("Skipping ICECandidatePool test on WASM")
	}

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOfferFlushStarted := make(chan SessionDescription)
	pcAnswerFlushStarted := make(chan SessionDescription)

	var offerOnce sync.Once
	var answerOnce sync.Once

	pcOffer, err := NewPeerConnection(Configuration{
		ICECandidatePoolSize: 1,
	})
	assert.NoError(t, err)

	// We need to create a data channel in order to set mid
	_, err = pcOffer.CreateDataChannel("initial_data_channel", nil)
	assert.NoError(t, err)

	pcOffer.OnICECandidate(func(i *ICECandidate) {
		offerOnce.Do(func() {
			close(pcOfferFlushStarted)
		})
	})

	// Assert that ICEGatheringState changes immediately
	assert.Eventually(t, func() bool {
		return pcOffer.ICEGatheringState() != ICEGatheringStateNew
	}, time.Second, 10*time.Millisecond, "ICEGatheringState should switch to Gathering or Complete immediately")

	// Assert that no events are fired before SetLocalDescription
	select {
	case <-pcOfferFlushStarted:
		assert.Fail(t, "Flush started before SetLocalDescription")
	case <-time.After(time.Second):
	}

	// Verify that candidates are flushed immediately after SetLocalDescription
	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))
	<-pcOfferFlushStarted

	// Create Answer PeerConnection
	pcAnswer, err := NewPeerConnection(Configuration{
		ICECandidatePoolSize: 1,
	})
	assert.NoError(t, err)

	pcAnswer.OnICECandidate(func(i *ICECandidate) {
		answerOnce.Do(func() {
			close(pcAnswerFlushStarted)
		})
	})

	// Assert that ICEGatheringState changes immediately
	assert.Eventually(t, func() bool {
		return pcAnswer.ICEGatheringState() != ICEGatheringStateNew
	}, time.Second, 10*time.Millisecond, "ICEGatheringState should switch to Gathering or Complete immediately")

	assert.NoError(t, pcAnswer.SetRemoteDescription(offer))
	select {
	case <-pcAnswerFlushStarted:
		assert.Fail(t, "Flush started before SetLocalDescription")
	case <-time.After(time.Second):
	}

	// Verify that candidates are flushed immediately after SetLocalDescription
	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)
	assert.NoError(t, pcAnswer.SetLocalDescription(answer))
	<-pcAnswerFlushStarted
	closePairNow(t, pcOffer, pcAnswer)
}

func TestSetICECandidatePoolSizeLarge(t *testing.T) {
	if runtime.GOARCH == "wasm" {
		t.Skip("Skipping ICECandidatePool test on WASM")
	}

	pc, err := NewPeerConnection(Configuration{
		ICECandidatePoolSize: 2,
	})
	assert.Nil(t, pc)
	assert.Equal(t, &rtcerr.NotSupportedError{Err: errICECandidatePoolSizeTooLarge}, err)
}

// Assert that SetRemoteDescription handles invalid states.
func TestSetRemoteDescriptionInvalid(t *testing.T) {
	t.Run("local-offer+SetRemoteDescription(Offer)", func(t *testing.T) {
		pc, err := NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		offer, err := pc.CreateOffer(nil)
		assert.NoError(t, err)

		assert.NoError(t, pc.SetLocalDescription(offer))
		assert.Error(t, pc.SetRemoteDescription(offer))

		assert.NoError(t, pc.Close())
	})
}

func TestAddTransceiver(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	for _, testCase := range []struct {
		expectSender, expectReceiver bool
		direction                    RTPTransceiverDirection
	}{
		{true, true, RTPTransceiverDirectionSendrecv},
		// Go and WASM diverge
		// {true, false, RTPTransceiverDirectionSendonly},
		// {false, true, RTPTransceiverDirectionRecvonly},
	} {
		pc, err := NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		transceiver, err := pc.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{
			Direction: testCase.direction,
		})
		assert.NoError(t, err)

		if testCase.expectReceiver {
			assert.NotNil(t, transceiver.Receiver())
		} else {
			assert.Nil(t, transceiver.Receiver())
		}

		if testCase.expectSender {
			assert.NotNil(t, transceiver.Sender())
		} else {
			assert.Nil(t, transceiver.Sender())
		}

		offer, err := pc.CreateOffer(nil)
		assert.NoError(t, err)

		assert.True(t, offerMediaHasDirection(offer, RTPCodecTypeVideo, testCase.direction))
		assert.NoError(t, pc.Close())
	}
}

// Assert that SCTPTransport -> DTLSTransport -> ICETransport works after connected.
func TestTransportChain(t *testing.T) {
	offer, answer, err := newPair()
	assert.NoError(t, err)

	peerConnectionsConnected := untilConnectionState(PeerConnectionStateConnected, offer, answer)
	assert.NoError(t, signalPair(offer, answer))
	peerConnectionsConnected.Wait()

	assert.NotNil(t, offer.SCTP().Transport().ICETransport())

	closePairNow(t, offer, answer)
}

// Assert that the PeerConnection closes via DTLS (and not ICE).
func TestDTLSClose(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	peerConnectionsConnected := untilConnectionState(PeerConnectionStateConnected, pcOffer, pcAnswer)

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	offerGatheringComplete := GatheringCompletePromise(pcOffer)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))
	<-offerGatheringComplete

	assert.NoError(t, pcAnswer.SetRemoteDescription(*pcOffer.LocalDescription()))

	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)

	answerGatheringComplete := GatheringCompletePromise(pcAnswer)
	assert.NoError(t, pcAnswer.SetLocalDescription(answer))
	<-answerGatheringComplete

	assert.NoError(t, pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()))

	peerConnectionsConnected.Wait()
	assert.NoError(t, pcOffer.Close())
}

func TestPeerConnection_SessionID(t *testing.T) {
	defer test.TimeOut(time.Second * 10).Stop()
	defer test.CheckRoutines(t)()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)
	var offerSessionID uint64
	var offerSessionVersion uint64
	var answerSessionID uint64
	var answerSessionVersion uint64
	for i := 0; i < 10; i++ {
		assert.NoError(t, signalPair(pcOffer, pcAnswer))
		var offer sdp.SessionDescription
		assert.NoError(t, offer.UnmarshalString(pcOffer.LocalDescription().SDP))

		sessionID := offer.Origin.SessionID
		sessionVersion := offer.Origin.SessionVersion

		if offerSessionID == 0 {
			offerSessionID = sessionID
			offerSessionVersion = sessionVersion
		} else {
			assert.Equalf(t, offerSessionID, sessionID, "offer[%v] session id mismatch", i)
			assert.Equalf(t, offerSessionVersion+1, sessionVersion, "offer[%v] session version mismatch", i)
			offerSessionVersion++
		}

		var answer sdp.SessionDescription
		assert.NoError(t, offer.UnmarshalString(pcAnswer.LocalDescription().SDP))

		sessionID = answer.Origin.SessionID
		sessionVersion = answer.Origin.SessionVersion

		if answerSessionID == 0 {
			answerSessionID = sessionID
			answerSessionVersion = sessionVersion
		} else {
			assert.Equalf(t, answerSessionID, sessionID, "answer[%v] session id mismatch", i)
			assert.Equalf(t, answerSessionVersion+1, sessionVersion, "answer[%v] session version mismatch", i)
			answerSessionVersion++
		}
	}
	closePairNow(t, pcOffer, pcAnswer)
}

func TestICETrickleCapabilityString(t *testing.T) {
	tests := []struct {
		value    ICETrickleCapability
		expected string
	}{
		{ICETrickleCapabilityUnknown, "unknown"},
		{ICETrickleCapabilitySupported, "supported"},
		{ICETrickleCapabilityUnsupported, "unsupported"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.value.String())
	}
}
