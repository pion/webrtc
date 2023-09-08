// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/sdp/v3"
	"github.com/pion/transport/v2/test"
	"github.com/pion/webrtc/v3/pkg/rtcerr"
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

func signalPairWithModification(pcOffer *PeerConnection, pcAnswer *PeerConnection, modificationFunc func(string) string) error {
	// Note(albrow): We need to create a data channel in order to trigger ICE
	// candidate gathering in the background for the JavaScript/Wasm bindings. If
	// we don't do this, the complete offer including ICE candidates will never be
	// generated.
	if _, err := pcOffer.CreateDataChannel("initial_data_channel", nil); err != nil {
		return err
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

func signalPair(pcOffer *PeerConnection, pcAnswer *PeerConnection) error {
	return signalPairWithModification(pcOffer, pcAnswer, func(sessionDescription string) string { return sessionDescription })
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
		ICECandidatePoolSize: 5,
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
					ICECandidatePoolSize: 5,
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
					ICETransportPolicy:   ICETransportPolicyAll,
					BundlePolicy:         BundlePolicyBalanced,
					RTCPMuxPolicy:        RTCPMuxPolicyRequire,
					ICECandidatePoolSize: 5,
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
	} {
		pc, err := test.init()
		if err != nil {
			t.Errorf("SetConfiguration %q: init failed: %v", test.name, err)
		}

		err = pc.SetConfiguration(test.config)
		if got, want := err, test.wantErr; !reflect.DeepEqual(got, want) {
			t.Errorf("SetConfiguration %q: err = %v, want %v", test.name, got, want)
		}

		assert.NoError(t, pc.Close())
	}
}

func TestPeerConnection_GetConfiguration(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	expected := Configuration{
		ICEServers:           []ICEServer{},
		ICETransportPolicy:   ICETransportPolicyAll,
		BundlePolicy:         BundlePolicyBalanced,
		RTCPMuxPolicy:        RTCPMuxPolicyRequire,
		ICECandidatePoolSize: 0,
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
	assert.NoError(t, pc.Close())
}

const minimalOffer = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
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
		if err != nil {
			t.Errorf("Case %d: got error: %v", i, err)
		}

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
	pcOffer.OnConnectionStateChange(func(callbackState PeerConnectionState) {
		if storedState := pcOffer.ConnectionState(); callbackState != storedState {
			t.Errorf("State in callback argument is different from ConnectionState(): callbackState=%s, storedState=%s", callbackState, storedState)
		}

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
		t.Fatalf("timed out waiting for one or more events handlers to be called (these *were* called: %+v)", wasCalled)
	}

	closePairNow(t, pcOffer, pcAnswer)
}

func TestMultipleOfferAnswer(t *testing.T) {
	firstPeerConn, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Errorf("New PeerConnection: got error: %v", err)
	}

	if _, err = firstPeerConn.CreateOffer(nil); err != nil {
		t.Errorf("First Offer: got error: %v", err)
	}
	if _, err = firstPeerConn.CreateOffer(nil); err != nil {
		t.Errorf("Second Offer: got error: %v", err)
	}

	secondPeerConn, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Errorf("New PeerConnection: got error: %v", err)
	}
	secondPeerConn.OnICECandidate(func(i *ICECandidate) {
	})

	if _, err = secondPeerConn.CreateOffer(nil); err != nil {
		t.Errorf("First Offer: got error: %v", err)
	}
	if _, err = secondPeerConn.CreateOffer(nil); err != nil {
		t.Errorf("Second Offer: got error: %v", err)
	}

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
	if err != nil {
		t.Error(err.Error())
	}

	desc := SessionDescription{
		Type: SDPTypeOffer,
		SDP:  sdpNoFingerprintInFirstMedia,
	}

	if err = pc.SetRemoteDescription(desc); err != nil {
		t.Error(err.Error())
	}

	assert.NoError(t, pc.Close())
}

func TestNegotiationNeeded(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

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

	pcAnswer.OnDataChannel(func(d *DataChannel) {
		wg.Done()
	})

	pcOffer.OnNegotiationNeeded(func() {
		offer, err := pcOffer.CreateOffer(nil)
		assert.NoError(t, err)

		offerGatheringComplete := GatheringCompletePromise(pcOffer)
		if err = pcOffer.SetLocalDescription(offer); err != nil {
			t.Error(err)
		}
		<-offerGatheringComplete
		if err = pcAnswer.SetRemoteDescription(*pcOffer.LocalDescription()); err != nil {
			t.Error(err)
		}

		answer, err := pcAnswer.CreateAnswer(nil)
		assert.NoError(t, err)

		answerGatheringComplete := GatheringCompletePromise(pcAnswer)
		if err = pcAnswer.SetLocalDescription(answer); err != nil {
			t.Error(err)
		}
		<-answerGatheringComplete
		if err = pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()); err != nil {
			t.Error(err)
		}
		wg.Done()
	})

	if _, err := pcOffer.CreateDataChannel("initial_data_channel_0", nil); err != nil {
		t.Error(err)
	}

	if _, err := pcOffer.CreateDataChannel("initial_data_channel_1", nil); err != nil {
		t.Error(err)
	}

	wg.Wait()

	closePairNow(t, pcOffer, pcAnswer)
}

// Assert that candidates are gathered by calling SetLocalDescription, not SetRemoteDescription
func TestGatherOnSetLocalDescription(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOfferGathered := make(chan SessionDescription)
	pcAnswerGathered := make(chan SessionDescription)

	s := SettingEngine{}
	api := NewAPI(WithSettingEngine(s))

	pcOffer, err := api.NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

	// We need to create a data channel in order to trigger ICE
	if _, err = pcOffer.CreateDataChannel("initial_data_channel", nil); err != nil {
		t.Error(err.Error())
	}

	pcOffer.OnICECandidate(func(i *ICECandidate) {
		if i == nil {
			close(pcOfferGathered)
		}
	})

	offer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		t.Error(err.Error())
	} else if err = pcOffer.SetLocalDescription(offer); err != nil {
		t.Error(err.Error())
	}

	<-pcOfferGathered

	pcAnswer, err := api.NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

	pcAnswer.OnICECandidate(func(i *ICECandidate) {
		if i == nil {
			close(pcAnswerGathered)
		}
	})

	if err = pcAnswer.SetRemoteDescription(offer); err != nil {
		t.Error(err.Error())
	}

	select {
	case <-pcAnswerGathered:
		t.Fatal("pcAnswer started gathering with no SetLocalDescription")
	// Gathering is async, not sure of a better way to catch this currently
	case <-time.After(3 * time.Second):
	}

	answer, err := pcAnswer.CreateAnswer(nil)
	if err != nil {
		t.Error(err.Error())
	} else if err = pcAnswer.SetLocalDescription(answer); err != nil {
		t.Error(err.Error())
	}
	<-pcAnswerGathered
	closePairNow(t, pcOffer, pcAnswer)
}

// Assert that SetRemoteDescription handles invalid states
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

// Assert that SCTPTransport -> DTLSTransport -> ICETransport works after connected
func TestTransportChain(t *testing.T) {
	offer, answer, err := newPair()
	assert.NoError(t, err)

	peerConnectionsConnected := untilConnectionState(PeerConnectionStateConnected, offer, answer)
	assert.NoError(t, signalPair(offer, answer))
	peerConnectionsConnected.Wait()

	assert.NotNil(t, offer.SCTP().Transport().ICETransport())

	closePairNow(t, offer, answer)
}
