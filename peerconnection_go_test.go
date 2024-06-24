// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/ice/v2"
	"github.com/pion/rtp"
	"github.com/pion/transport/v2/test"
	"github.com/pion/transport/v2/vnet"
	"github.com/pion/webrtc/v3/internal/util"
	"github.com/pion/webrtc/v3/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
)

// newPair creates two new peer connections (an offerer and an answerer) using
// the api.
func (api *API) newPair(cfg Configuration) (pcOffer *PeerConnection, pcAnswer *PeerConnection, err error) {
	pca, err := api.NewPeerConnection(cfg)
	if err != nil {
		return nil, nil, err
	}

	pcb, err := api.NewPeerConnection(cfg)
	if err != nil {
		return nil, nil, err
	}

	return pca, pcb, nil
}

func TestNew_Go(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	t.Run("Success", func(t *testing.T) {
		secretKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		assert.Nil(t, err)

		certificate, err := GenerateCertificate(secretKey)
		assert.Nil(t, err)

		pc, err := api.NewPeerConnection(Configuration{
			ICEServers: []ICEServer{
				{
					URLs: []string{
						"stun:stun.l.google.com:19302",
						"turns:google.de?transport=tcp",
					},
					Username: "unittest",
					Credential: OAuthCredential{
						MACKey:      "WmtzanB3ZW9peFhtdm42NzUzNG0=",
						AccessToken: "AAwg3kPHWPfvk9bDFL936wYvkoctMADzQ==",
					},
					CredentialType: ICECredentialTypeOauth,
				},
			},
			ICETransportPolicy:   ICETransportPolicyRelay,
			BundlePolicy:         BundlePolicyMaxCompat,
			RTCPMuxPolicy:        RTCPMuxPolicyNegotiate,
			PeerIdentity:         "unittest",
			Certificates:         []Certificate{*certificate},
			ICECandidatePoolSize: 5,
		})
		assert.Nil(t, err)
		assert.NotNil(t, pc)
		assert.NoError(t, pc.Close())
	})
	t.Run("Failure", func(t *testing.T) {
		testCases := []struct {
			initialize  func() (*PeerConnection, error)
			expectedErr error
		}{
			{func() (*PeerConnection, error) {
				secretKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				assert.Nil(t, err)

				certificate, err := NewCertificate(secretKey, x509.Certificate{
					Version:      2,
					SerialNumber: big.NewInt(1653),
					NotBefore:    time.Now().AddDate(0, -2, 0),
					NotAfter:     time.Now().AddDate(0, -1, 0),
				})
				assert.Nil(t, err)

				return api.NewPeerConnection(Configuration{
					Certificates: []Certificate{*certificate},
				})
			}, &rtcerr.InvalidAccessError{Err: ErrCertificateExpired}},
			{func() (*PeerConnection, error) {
				return api.NewPeerConnection(Configuration{
					ICEServers: []ICEServer{
						{
							URLs: []string{
								"stun:stun.l.google.com:19302",
								"turns:google.de?transport=tcp",
							},
							Username: "unittest",
						},
					},
				})
			}, &rtcerr.InvalidAccessError{Err: ErrNoTurnCredentials}},
		}

		for i, testCase := range testCases {
			pc, err := testCase.initialize()
			assert.EqualError(t, err, testCase.expectedErr.Error(),
				"testCase: %d %v", i, testCase,
			)
			if pc != nil {
				assert.NoError(t, pc.Close())
			}
		}
	})
	t.Run("ICEServers_Copy", func(t *testing.T) {
		const expectedURL = "stun:stun.l.google.com:19302?foo=bar"
		const expectedUsername = "username"
		const expectedPassword = "password"

		cfg := Configuration{
			ICEServers: []ICEServer{
				{
					URLs:       []string{expectedURL},
					Username:   expectedUsername,
					Credential: expectedPassword,
				},
			},
		}
		pc, err := api.NewPeerConnection(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, pc)

		pc.configuration.ICEServers[0].Username = util.MathRandAlpha(15) // Tests doesn't need crypto random
		pc.configuration.ICEServers[0].Credential = util.MathRandAlpha(15)
		pc.configuration.ICEServers[0].URLs[0] = util.MathRandAlpha(15)

		assert.Equal(t, expectedUsername, cfg.ICEServers[0].Username)
		assert.Equal(t, expectedPassword, cfg.ICEServers[0].Credential)
		assert.Equal(t, expectedURL, cfg.ICEServers[0].URLs[0])

		assert.NoError(t, pc.Close())
	})
}

func TestPeerConnection_SetConfiguration_Go(t *testing.T) {
	// Note: this test includes all SetConfiguration features that are supported
	// by Go but not the WASM bindings, namely: ICEServer.Credential,
	// ICEServer.CredentialType, and Certificates.
	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()

	secretKey1, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)

	certificate1, err := GenerateCertificate(secretKey1)
	assert.Nil(t, err)

	secretKey2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)

	certificate2, err := GenerateCertificate(secretKey2)
	assert.Nil(t, err)

	for _, test := range []struct {
		name    string
		init    func() (*PeerConnection, error)
		config  Configuration
		wantErr error
	}{
		{
			name: "valid",
			init: func() (*PeerConnection, error) {
				pc, err := api.NewPeerConnection(Configuration{
					PeerIdentity:         "unittest",
					Certificates:         []Certificate{*certificate1},
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
								"turns:google.de?transport=tcp",
							},
							Username: "unittest",
							Credential: OAuthCredential{
								MACKey:      "WmtzanB3ZW9peFhtdm42NzUzNG0=",
								AccessToken: "AAwg3kPHWPfvk9bDFL936wYvkoctMADzQ==",
							},
							CredentialType: ICECredentialTypeOauth,
						},
					},
					ICETransportPolicy:   ICETransportPolicyAll,
					BundlePolicy:         BundlePolicyBalanced,
					RTCPMuxPolicy:        RTCPMuxPolicyRequire,
					PeerIdentity:         "unittest",
					Certificates:         []Certificate{*certificate1},
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
			name: "update multiple certificates",
			init: func() (*PeerConnection, error) {
				return api.NewPeerConnection(Configuration{})
			},
			config: Configuration{
				Certificates: []Certificate{*certificate1, *certificate2},
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingCertificates},
		},
		{
			name: "update certificate",
			init: func() (*PeerConnection, error) {
				return api.NewPeerConnection(Configuration{})
			},
			config: Configuration{
				Certificates: []Certificate{*certificate1},
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingCertificates},
		},
		{
			name: "update ICEServers, no TURN credentials",
			init: func() (*PeerConnection, error) {
				return NewPeerConnection(Configuration{})
			},
			config: Configuration{
				ICEServers: []ICEServer{
					{
						URLs: []string{
							"stun:stun.l.google.com:19302",
							"turns:google.de?transport=tcp",
						},
						Username: "unittest",
					},
				},
			},
			wantErr: &rtcerr.InvalidAccessError{Err: ErrNoTurnCredentials},
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

func TestPeerConnection_EventHandlers_Go(t *testing.T) {
	lim := test.TimeOut(time.Second * 5)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	// Note: When testing the Go event handlers we peer into the state a bit more
	// than what is possible for the environment agnostic (Go or WASM/JavaScript)
	// EventHandlers test.
	api := NewAPI()
	pc, err := api.NewPeerConnection(Configuration{})
	assert.Nil(t, err)

	onTrackCalled := make(chan struct{})
	onICEConnectionStateChangeCalled := make(chan struct{})
	onDataChannelCalled := make(chan struct{})

	// Verify that the noop case works
	assert.NotPanics(t, func() { pc.onTrack(nil, nil) })
	assert.NotPanics(t, func() { pc.onICEConnectionStateChange(ICEConnectionStateNew) })

	pc.OnTrack(func(t *TrackRemote, r *RTPReceiver) {
		close(onTrackCalled)
	})

	pc.OnICEConnectionStateChange(func(cs ICEConnectionState) {
		close(onICEConnectionStateChangeCalled)
	})

	pc.OnDataChannel(func(dc *DataChannel) {
		// Questions:
		//  (1) How come this callback is made with dc being nil?
		//  (2) How come this callback is made without CreateDataChannel?
		if dc != nil {
			close(onDataChannelCalled)
		}
	})

	// Verify that the handlers deal with nil inputs
	assert.NotPanics(t, func() { pc.onTrack(nil, nil) })
	assert.NotPanics(t, func() { go pc.onDataChannelHandler(nil) })

	// Verify that the set handlers are called
	assert.NotPanics(t, func() { pc.onTrack(&TrackRemote{}, &RTPReceiver{}) })
	assert.NotPanics(t, func() { pc.onICEConnectionStateChange(ICEConnectionStateNew) })
	assert.NotPanics(t, func() { go pc.onDataChannelHandler(&DataChannel{api: api}) })

	<-onTrackCalled
	<-onICEConnectionStateChangeCalled
	<-onDataChannelCalled
	assert.NoError(t, pc.Close())
}

// This test asserts that nothing deadlocks we try to shutdown when DTLS is in flight
// We ensure that DTLS is in flight by removing the mux func for it, so all inbound DTLS is lost
func TestPeerConnection_ShutdownNoDTLS(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	offerPC, answerPC, err := api.newPair(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	// Drop all incoming DTLS traffic
	dropAllDTLS := func([]byte) bool {
		return false
	}
	offerPC.dtlsTransport.dtlsMatcher = dropAllDTLS
	answerPC.dtlsTransport.dtlsMatcher = dropAllDTLS

	if err = signalPair(offerPC, answerPC); err != nil {
		t.Fatal(err)
	}

	iceComplete := make(chan interface{})
	answerPC.OnICEConnectionStateChange(func(iceState ICEConnectionState) {
		if iceState == ICEConnectionStateConnected {
			time.Sleep(time.Second) // Give time for DTLS to start

			select {
			case <-iceComplete:
			default:
				close(iceComplete)
			}
		}
	})

	<-iceComplete
	closePairNow(t, offerPC, answerPC)
}

func TestPeerConnection_PropertyGetters(t *testing.T) {
	pc := &PeerConnection{
		currentLocalDescription:  &SessionDescription{},
		pendingLocalDescription:  &SessionDescription{},
		currentRemoteDescription: &SessionDescription{},
		pendingRemoteDescription: &SessionDescription{},
		signalingState:           SignalingStateHaveLocalOffer,
	}
	pc.iceConnectionState.Store(ICEConnectionStateChecking)
	pc.connectionState.Store(PeerConnectionStateConnecting)

	assert.Equal(t, pc.currentLocalDescription, pc.CurrentLocalDescription(), "should match")
	assert.Equal(t, pc.pendingLocalDescription, pc.PendingLocalDescription(), "should match")
	assert.Equal(t, pc.currentRemoteDescription, pc.CurrentRemoteDescription(), "should match")
	assert.Equal(t, pc.pendingRemoteDescription, pc.PendingRemoteDescription(), "should match")
	assert.Equal(t, pc.signalingState, pc.SignalingState(), "should match")
	assert.Equal(t, pc.iceConnectionState.Load(), pc.ICEConnectionState(), "should match")
	assert.Equal(t, pc.connectionState.Load(), pc.ConnectionState(), "should match")
}

func TestPeerConnection_AnswerWithoutOffer(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Errorf("New PeerConnection: got error: %v", err)
	}
	_, err = pc.CreateAnswer(nil)
	if !reflect.DeepEqual(&rtcerr.InvalidStateError{Err: ErrNoRemoteDescription}, err) {
		t.Errorf("CreateAnswer without RemoteDescription: got error: %v", err)
	}

	assert.NoError(t, pc.Close())
}

func TestPeerConnection_AnswerWithClosedConnection(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	offerPeerConn, answerPeerConn, err := newPair()
	assert.NoError(t, err)

	inChecking, inCheckingCancel := context.WithCancel(context.Background())
	answerPeerConn.OnICEConnectionStateChange(func(i ICEConnectionState) {
		if i == ICEConnectionStateChecking {
			inCheckingCancel()
		}
	})

	_, err = offerPeerConn.CreateDataChannel("test-channel", nil)
	assert.NoError(t, err)

	offer, err := offerPeerConn.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, offerPeerConn.SetLocalDescription(offer))

	assert.NoError(t, offerPeerConn.Close())

	assert.NoError(t, answerPeerConn.SetRemoteDescription(offer))

	<-inChecking.Done()
	assert.NoError(t, answerPeerConn.Close())

	_, err = answerPeerConn.CreateAnswer(nil)
	assert.Equal(t, err, &rtcerr.InvalidStateError{Err: ErrConnectionClosed})
}

func TestPeerConnection_satisfyTypeAndDirection(t *testing.T) {
	createTransceiver := func(kind RTPCodecType, direction RTPTransceiverDirection) *RTPTransceiver {
		r := &RTPTransceiver{kind: kind}
		r.setDirection(direction)

		return r
	}

	for _, test := range []struct {
		name string

		kinds      []RTPCodecType
		directions []RTPTransceiverDirection

		localTransceivers []*RTPTransceiver
		want              []*RTPTransceiver
	}{
		{
			"Audio and Video Transceivers can not satisfy each other",
			[]RTPCodecType{RTPCodecTypeVideo},
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendrecv},
			[]*RTPTransceiver{createTransceiver(RTPCodecTypeAudio, RTPTransceiverDirectionSendrecv)},
			[]*RTPTransceiver{nil},
		},
		{
			"No local Transceivers, every remote should get nil",
			[]RTPCodecType{RTPCodecTypeVideo, RTPCodecTypeAudio, RTPCodecTypeVideo, RTPCodecTypeVideo},
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendrecv, RTPTransceiverDirectionRecvonly, RTPTransceiverDirectionSendonly, RTPTransceiverDirectionInactive},

			[]*RTPTransceiver{},

			[]*RTPTransceiver{
				nil,
				nil,
				nil,
				nil,
			},
		},
		{
			"Local Recv can satisfy remote SendRecv",
			[]RTPCodecType{RTPCodecTypeVideo},
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendrecv},

			[]*RTPTransceiver{createTransceiver(RTPCodecTypeVideo, RTPTransceiverDirectionRecvonly)},

			[]*RTPTransceiver{createTransceiver(RTPCodecTypeVideo, RTPTransceiverDirectionRecvonly)},
		},
		{
			"Don't satisfy a Sendonly with a SendRecv, later SendRecv will be marked as Inactive",
			[]RTPCodecType{RTPCodecTypeVideo, RTPCodecTypeVideo},
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendonly, RTPTransceiverDirectionSendrecv},

			[]*RTPTransceiver{
				createTransceiver(RTPCodecTypeVideo, RTPTransceiverDirectionSendrecv),
				createTransceiver(RTPCodecTypeVideo, RTPTransceiverDirectionRecvonly),
			},

			[]*RTPTransceiver{
				createTransceiver(RTPCodecTypeVideo, RTPTransceiverDirectionRecvonly),
				createTransceiver(RTPCodecTypeVideo, RTPTransceiverDirectionSendrecv),
			},
		},
	} {
		if len(test.kinds) != len(test.directions) {
			t.Fatal("Kinds and Directions must be the same length")
		}

		got := []*RTPTransceiver{}
		for i := range test.kinds {
			res, filteredLocalTransceivers := satisfyTypeAndDirection(test.kinds[i], test.directions[i], test.localTransceivers)

			got = append(got, res)
			test.localTransceivers = filteredLocalTransceivers
		}

		if !reflect.DeepEqual(got, test.want) {
			gotStr := ""
			for _, t := range got {
				gotStr += fmt.Sprintf("%+v\n", t)
			}

			wantStr := ""
			for _, t := range test.want {
				wantStr += fmt.Sprintf("%+v\n", t)
			}
			t.Errorf("satisfyTypeAndDirection %q: \ngot\n%s \nwant\n%s", test.name, gotStr, wantStr)
		}
	}
}

func TestOneAttrKeyConnectionSetupPerMediaDescriptionInSDP(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = pc.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	_, err = pc.AddTransceiverFromKind(RTPCodecTypeAudio)
	assert.NoError(t, err)

	_, err = pc.AddTransceiverFromKind(RTPCodecTypeAudio)
	assert.NoError(t, err)

	_, err = pc.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	sdp, err := pc.CreateOffer(nil)
	assert.NoError(t, err)

	re := regexp.MustCompile(`a=setup:[[:alpha:]]+`)

	matches := re.FindAllStringIndex(sdp.SDP, -1)

	assert.Len(t, matches, 4)
	assert.NoError(t, pc.Close())
}

func TestPeerConnection_IceLite(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	connectTwoAgents := func(offerIsLite, answerisLite bool) {
		offerSettingEngine := SettingEngine{}
		offerSettingEngine.SetLite(offerIsLite)
		offerPC, err := NewAPI(WithSettingEngine(offerSettingEngine)).NewPeerConnection(Configuration{})
		if err != nil {
			t.Fatal(err)
		}

		answerSettingEngine := SettingEngine{}
		answerSettingEngine.SetLite(answerisLite)
		answerPC, err := NewAPI(WithSettingEngine(answerSettingEngine)).NewPeerConnection(Configuration{})
		if err != nil {
			t.Fatal(err)
		}

		if err = signalPair(offerPC, answerPC); err != nil {
			t.Fatal(err)
		}

		dataChannelOpen := make(chan interface{})
		answerPC.OnDataChannel(func(_ *DataChannel) {
			close(dataChannelOpen)
		})

		<-dataChannelOpen
		closePairNow(t, offerPC, answerPC)
	}

	t.Run("Offerer", func(t *testing.T) {
		connectTwoAgents(true, false)
	})

	t.Run("Answerer", func(t *testing.T) {
		connectTwoAgents(false, true)
	})

	t.Run("Both", func(t *testing.T) {
		connectTwoAgents(true, true)
	})
}

func TestOnICEGatheringStateChange(t *testing.T) {
	seenGathering := &atomicBool{}
	seenComplete := &atomicBool{}

	seenGatheringAndComplete := make(chan interface{})
	seenClosed := make(chan interface{})

	peerConn, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	var onStateChange func(s ICEGathererState)
	onStateChange = func(s ICEGathererState) {
		// Access to ICEGatherer in the callback must not cause dead lock.
		peerConn.OnICEGatheringStateChange(onStateChange)
		if state := peerConn.iceGatherer.State(); state != s {
			t.Errorf("State change callback argument (%s) and State() (%s) result differs",
				s, state,
			)
		}

		switch s { // nolint:exhaustive
		case ICEGathererStateClosed:
			close(seenClosed)
			return
		case ICEGathererStateGathering:
			if seenComplete.get() {
				t.Error("Completed before gathering")
			}
			seenGathering.set(true)
		case ICEGathererStateComplete:
			seenComplete.set(true)
		}

		if seenGathering.get() && seenComplete.get() {
			close(seenGatheringAndComplete)
		}
	}
	peerConn.OnICEGatheringStateChange(onStateChange)

	offer, err := peerConn.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, peerConn.SetLocalDescription(offer))

	select {
	case <-time.After(time.Second * 10):
		t.Fatal("Gathering and Complete were never seen")
	case <-seenClosed:
		t.Fatal("Closed before PeerConnection Close")
	case <-seenGatheringAndComplete:
	}

	assert.NoError(t, peerConn.Close())

	select {
	case <-time.After(time.Second * 10):
		t.Fatal("Closed was never seen")
	case <-seenClosed:
	}
}

// Assert Trickle ICE behaviors
func TestPeerConnectionTrickle(t *testing.T) {
	offerPC, answerPC, err := newPair()
	assert.NoError(t, err)

	_, err = offerPC.CreateDataChannel("test-channel", nil)
	assert.NoError(t, err)

	addOrCacheCandidate := func(pc *PeerConnection, c *ICECandidate, candidateCache []ICECandidateInit) []ICECandidateInit {
		if c == nil {
			return candidateCache
		}

		if pc.RemoteDescription() == nil {
			return append(candidateCache, c.ToJSON())
		}

		assert.NoError(t, pc.AddICECandidate(c.ToJSON()))
		return candidateCache
	}

	candidateLock := sync.RWMutex{}
	var offerCandidateDone, answerCandidateDone bool

	cachedOfferCandidates := []ICECandidateInit{}
	offerPC.OnICECandidate(func(c *ICECandidate) {
		if offerCandidateDone {
			t.Error("Received OnICECandidate after finishing gathering")
		}
		if c == nil {
			offerCandidateDone = true
		}

		candidateLock.Lock()
		defer candidateLock.Unlock()

		cachedOfferCandidates = addOrCacheCandidate(answerPC, c, cachedOfferCandidates)
	})

	cachedAnswerCandidates := []ICECandidateInit{}
	answerPC.OnICECandidate(func(c *ICECandidate) {
		if answerCandidateDone {
			t.Error("Received OnICECandidate after finishing gathering")
		}
		if c == nil {
			answerCandidateDone = true
		}

		candidateLock.Lock()
		defer candidateLock.Unlock()

		cachedAnswerCandidates = addOrCacheCandidate(offerPC, c, cachedAnswerCandidates)
	})

	offerPCConnected, offerPCConnectedCancel := context.WithCancel(context.Background())
	offerPC.OnICEConnectionStateChange(func(i ICEConnectionState) {
		if i == ICEConnectionStateConnected {
			offerPCConnectedCancel()
		}
	})

	answerPCConnected, answerPCConnectedCancel := context.WithCancel(context.Background())
	answerPC.OnICEConnectionStateChange(func(i ICEConnectionState) {
		if i == ICEConnectionStateConnected {
			answerPCConnectedCancel()
		}
	})

	offer, err := offerPC.CreateOffer(nil)
	assert.NoError(t, err)

	assert.NoError(t, offerPC.SetLocalDescription(offer))
	assert.NoError(t, answerPC.SetRemoteDescription(offer))

	answer, err := answerPC.CreateAnswer(nil)
	assert.NoError(t, err)

	assert.NoError(t, answerPC.SetLocalDescription(answer))
	assert.NoError(t, offerPC.SetRemoteDescription(answer))

	candidateLock.Lock()
	for _, c := range cachedAnswerCandidates {
		assert.NoError(t, offerPC.AddICECandidate(c))
	}
	for _, c := range cachedOfferCandidates {
		assert.NoError(t, answerPC.AddICECandidate(c))
	}
	candidateLock.Unlock()

	<-answerPCConnected.Done()
	<-offerPCConnected.Done()
	closePairNow(t, offerPC, answerPC)
}

// Issue #1121, assert populateLocalCandidates doesn't mutate
func TestPopulateLocalCandidates(t *testing.T) {
	t.Run("PendingLocalDescription shouldn't add extra mutations", func(t *testing.T) {
		pc, err := NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		offer, err := pc.CreateOffer(nil)
		assert.NoError(t, err)

		offerGatheringComplete := GatheringCompletePromise(pc)
		assert.NoError(t, pc.SetLocalDescription(offer))
		<-offerGatheringComplete

		assert.Equal(t, pc.PendingLocalDescription(), pc.PendingLocalDescription())
		assert.NoError(t, pc.Close())
	})

	t.Run("end-of-candidates only when gathering is complete", func(t *testing.T) {
		pc, err := NewAPI().NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		_, err = pc.CreateDataChannel("test-channel", nil)
		assert.NoError(t, err)

		offer, err := pc.CreateOffer(nil)
		assert.NoError(t, err)
		assert.NotContains(t, offer.SDP, "a=candidate")
		assert.NotContains(t, offer.SDP, "a=end-of-candidates")

		offerGatheringComplete := GatheringCompletePromise(pc)
		assert.NoError(t, pc.SetLocalDescription(offer))
		<-offerGatheringComplete

		assert.Contains(t, pc.PendingLocalDescription().SDP, "a=candidate")
		assert.Contains(t, pc.PendingLocalDescription().SDP, "a=end-of-candidates")

		assert.NoError(t, pc.Close())
	})
}

// Assert that two agents that only generate mDNS candidates can connect
func TestMulticastDNSCandidates(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	s := SettingEngine{}
	s.SetICEMulticastDNSMode(ice.MulticastDNSModeQueryAndGather)

	pcOffer, pcAnswer, err := NewAPI(WithSettingEngine(s)).newPair(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	onDataChannel, onDataChannelCancel := context.WithCancel(context.Background())
	pcAnswer.OnDataChannel(func(d *DataChannel) {
		onDataChannelCancel()
	})
	<-onDataChannel.Done()

	closePairNow(t, pcOffer, pcAnswer)
}

func TestICERestart(t *testing.T) {
	extractCandidates := func(sdp string) (candidates []string) {
		sc := bufio.NewScanner(strings.NewReader(sdp))
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "a=candidate:") {
				candidates = append(candidates, sc.Text())
			}
		}

		return
	}

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	offerPC, answerPC, err := newPair()
	assert.NoError(t, err)

	var connectedWaitGroup sync.WaitGroup
	connectedWaitGroup.Add(2)

	offerPC.OnICEConnectionStateChange(func(state ICEConnectionState) {
		if state == ICEConnectionStateConnected {
			connectedWaitGroup.Done()
		}
	})
	answerPC.OnICEConnectionStateChange(func(state ICEConnectionState) {
		if state == ICEConnectionStateConnected {
			connectedWaitGroup.Done()
		}
	})

	// Connect two PeerConnections and block until ICEConnectionStateConnected
	assert.NoError(t, signalPair(offerPC, answerPC))
	connectedWaitGroup.Wait()

	// Store candidates from first Offer/Answer, compare later to make sure we re-gathered
	firstOfferCandidates := extractCandidates(offerPC.LocalDescription().SDP)
	firstAnswerCandidates := extractCandidates(answerPC.LocalDescription().SDP)

	// Use Trickle ICE for ICE Restart
	offerPC.OnICECandidate(func(c *ICECandidate) {
		if c != nil {
			assert.NoError(t, answerPC.AddICECandidate(c.ToJSON()))
		}
	})

	answerPC.OnICECandidate(func(c *ICECandidate) {
		if c != nil {
			assert.NoError(t, offerPC.AddICECandidate(c.ToJSON()))
		}
	})

	// Re-signal with ICE Restart, block until ICEConnectionStateConnected
	connectedWaitGroup.Add(2)
	offer, err := offerPC.CreateOffer(&OfferOptions{ICERestart: true})
	assert.NoError(t, err)

	assert.NoError(t, offerPC.SetLocalDescription(offer))
	assert.NoError(t, answerPC.SetRemoteDescription(offer))

	answer, err := answerPC.CreateAnswer(nil)
	assert.NoError(t, err)

	assert.NoError(t, answerPC.SetLocalDescription(answer))
	assert.NoError(t, offerPC.SetRemoteDescription(answer))

	// Block until we have connected again
	connectedWaitGroup.Wait()

	// Compare ICE Candidates across each run, fail if they haven't changed
	assert.NotEqual(t, firstOfferCandidates, extractCandidates(offerPC.LocalDescription().SDP))
	assert.NotEqual(t, firstAnswerCandidates, extractCandidates(answerPC.LocalDescription().SDP))
	closePairNow(t, offerPC, answerPC)
}

// Assert error handling when an Agent is restart
func TestICERestart_Error_Handling(t *testing.T) {
	iceStates := make(chan ICEConnectionState, 100)
	blockUntilICEState := func(wantedState ICEConnectionState) {
		stateCount := 0
		for i := range iceStates {
			if i == wantedState {
				stateCount++
			}

			if stateCount == 2 {
				return
			}
		}
	}

	connectWithICERestart := func(offerPeerConnection, answerPeerConnection *PeerConnection) {
		offer, err := offerPeerConnection.CreateOffer(&OfferOptions{ICERestart: true})
		assert.NoError(t, err)

		assert.NoError(t, offerPeerConnection.SetLocalDescription(offer))
		assert.NoError(t, answerPeerConnection.SetRemoteDescription(*offerPeerConnection.LocalDescription()))

		answer, err := answerPeerConnection.CreateAnswer(nil)
		assert.NoError(t, err)

		assert.NoError(t, answerPeerConnection.SetLocalDescription(answer))
		assert.NoError(t, offerPeerConnection.SetRemoteDescription(*answerPeerConnection.LocalDescription()))
	}

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	offerPeerConnection, answerPeerConnection, wan := createVNetPair(t)

	pushICEState := func(i ICEConnectionState) { iceStates <- i }
	offerPeerConnection.OnICEConnectionStateChange(pushICEState)
	answerPeerConnection.OnICEConnectionStateChange(pushICEState)

	keepPackets := &atomicBool{}
	keepPackets.set(true)

	// Add a filter that monitors the traffic on the router
	wan.AddChunkFilter(func(c vnet.Chunk) bool {
		return keepPackets.get()
	})

	const testMessage = "testMessage"

	d, err := answerPeerConnection.CreateDataChannel("foo", nil)
	assert.NoError(t, err)

	dataChannelMessages := make(chan string, 100)
	d.OnMessage(func(m DataChannelMessage) {
		dataChannelMessages <- string(m.Data)
	})

	dataChannelAnswerer := make(chan *DataChannel)
	offerPeerConnection.OnDataChannel(func(d *DataChannel) {
		d.OnOpen(func() {
			dataChannelAnswerer <- d
		})
	})

	// Connect and Assert we have connected
	assert.NoError(t, signalPair(offerPeerConnection, answerPeerConnection))
	blockUntilICEState(ICEConnectionStateConnected)

	offerPeerConnection.OnICECandidate(func(c *ICECandidate) {
		if c != nil {
			assert.NoError(t, answerPeerConnection.AddICECandidate(c.ToJSON()))
		}
	})

	answerPeerConnection.OnICECandidate(func(c *ICECandidate) {
		if c != nil {
			assert.NoError(t, offerPeerConnection.AddICECandidate(c.ToJSON()))
		}
	})

	dataChannel := <-dataChannelAnswerer
	assert.NoError(t, dataChannel.SendText(testMessage))
	assert.Equal(t, testMessage, <-dataChannelMessages)

	// Drop all packets, assert we have disconnected
	// and send a DataChannel message when disconnected
	keepPackets.set(false)
	blockUntilICEState(ICEConnectionStateFailed)
	assert.NoError(t, dataChannel.SendText(testMessage))

	// ICE Restart and assert we have reconnected
	// block until our DataChannel message is delivered
	keepPackets.set(true)
	connectWithICERestart(offerPeerConnection, answerPeerConnection)
	blockUntilICEState(ICEConnectionStateConnected)
	assert.Equal(t, testMessage, <-dataChannelMessages)

	assert.NoError(t, wan.Stop())
	closePairNow(t, offerPeerConnection, answerPeerConnection)
}

type trackRecords struct {
	mu               sync.Mutex
	trackIDs         map[string]struct{}
	receivedTrackIDs map[string]struct{}
}

func (r *trackRecords) newTrack() (*TrackLocalStaticRTP, error) {
	trackID := fmt.Sprintf("pion-track-%d", len(r.trackIDs))
	track, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, trackID, "pion")
	r.trackIDs[trackID] = struct{}{}
	return track, err
}

func (r *trackRecords) handleTrack(t *TrackRemote, _ *RTPReceiver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tID := t.ID()
	if _, exist := r.trackIDs[tID]; exist {
		r.receivedTrackIDs[tID] = struct{}{}
	}
}

func (r *trackRecords) remains() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.trackIDs) - len(r.receivedTrackIDs)
}

// This test assure that all track events emits.
func TestPeerConnection_MassiveTracks(t *testing.T) {
	var (
		api   = NewAPI()
		tRecs = &trackRecords{
			trackIDs:         make(map[string]struct{}),
			receivedTrackIDs: make(map[string]struct{}),
		}
		tracks          = []*TrackLocalStaticRTP{}
		trackCount      = 256
		pingInterval    = 1 * time.Second
		noiseInterval   = 100 * time.Microsecond
		timeoutDuration = 20 * time.Second
		rawPkt          = []byte{
			0x90, 0xe0, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
			0x27, 0x82, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x98, 0x36, 0xbe, 0x88, 0x9e,
		}
		samplePkt = &rtp.Packet{
			Header: rtp.Header{
				Marker:           true,
				Extension:        false,
				ExtensionProfile: 1,
				Version:          2,
				SequenceNumber:   27023,
				Timestamp:        3653407706,
				CSRC:             []uint32{},
			},
			Payload: rawPkt[20:],
		}
		connected = make(chan struct{})
		stopped   = make(chan struct{})
	)
	assert.NoError(t, api.mediaEngine.RegisterDefaultCodecs())
	offerPC, answerPC, err := api.newPair(Configuration{})
	assert.NoError(t, err)
	// Create massive tracks.
	for range make([]struct{}, trackCount) {
		track, err := tRecs.newTrack()
		assert.NoError(t, err)
		_, err = offerPC.AddTrack(track)
		assert.NoError(t, err)
		tracks = append(tracks, track)
	}
	answerPC.OnTrack(tRecs.handleTrack)
	offerPC.OnICEConnectionStateChange(func(s ICEConnectionState) {
		if s == ICEConnectionStateConnected {
			close(connected)
		}
	})
	// A routine to periodically call GetTransceivers. This action might cause
	// the deadlock and prevent track event to emit.
	go func() {
		for {
			answerPC.GetTransceivers()
			time.Sleep(noiseInterval)
			select {
			case <-stopped:
				return
			default:
			}
		}
	}()
	assert.NoError(t, signalPair(offerPC, answerPC))
	// Send a RTP packets to each track to trigger track event after connected.
	<-connected
	time.Sleep(1 * time.Second)
	for _, track := range tracks {
		assert.NoError(t, track.WriteRTP(samplePkt))
	}
	// Ping trackRecords to see if any track event not received yet.
	tooLong := time.After(timeoutDuration)
	for {
		remains := tRecs.remains()
		if remains == 0 {
			break
		}
		t.Log("remain tracks", remains)
		time.Sleep(pingInterval)
		select {
		case <-tooLong:
			t.Error("unable to receive all track events in time")
		default:
		}
	}
	close(stopped)
	closePairNow(t, offerPC, answerPC)
}

func TestEmptyCandidate(t *testing.T) {
	testCases := []struct {
		ICECandidate ICECandidateInit
		expectError  bool
	}{
		{ICECandidateInit{"", nil, nil, nil}, false},
		{ICECandidateInit{
			"211962667 1 udp 2122194687 10.0.3.1 40864 typ host generation 0",
			nil, nil, nil,
		}, false},
		{ICECandidateInit{
			"1234567",
			nil, nil, nil,
		}, true},
	}

	for i, testCase := range testCases {
		peerConn, err := NewPeerConnection(Configuration{})
		if err != nil {
			t.Errorf("Case %d: got error: %v", i, err)
		}

		err = peerConn.SetRemoteDescription(SessionDescription{Type: SDPTypeOffer, SDP: minimalOffer})
		if err != nil {
			t.Errorf("Case %d: got error: %v", i, err)
		}

		if testCase.expectError {
			assert.Error(t, peerConn.AddICECandidate(testCase.ICECandidate))
		} else {
			assert.NoError(t, peerConn.AddICECandidate(testCase.ICECandidate))
		}

		assert.NoError(t, peerConn.Close())
	}
}

const liteOffer = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
a=msid-semantic: WMS
a=ice-lite
m=application 47299 DTLS/SCTP 5000
c=IN IP4 192.168.20.129
a=ice-ufrag:1/MvHwjAyVf27aLu
a=ice-pwd:3dBU7cFOBl120v33cynDvN1E
a=fingerprint:sha-256 75:74:5A:A6:A4:E5:52:F4:A7:67:4C:01:C7:EE:91:3F:21:3D:A2:E3:53:7B:6F:30:86:F2:30:AA:65:FB:04:24
a=mid:data
`

// this test asserts that if an ice-lite offer is received,
// pion will take the ICE-CONTROLLING role
func TestICELite(t *testing.T) {
	peerConnection, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, peerConnection.SetRemoteDescription(
		SessionDescription{SDP: liteOffer, Type: SDPTypeOffer},
	))

	SDPAnswer, err := peerConnection.CreateAnswer(nil)
	assert.NoError(t, err)

	assert.NoError(t, peerConnection.SetLocalDescription(SDPAnswer))

	assert.Equal(t, ICERoleControlling, peerConnection.iceTransport.Role(),
		"pion did not set state to ICE-CONTROLLED against ice-light offer")

	assert.NoError(t, peerConnection.Close())
}

func TestPeerConnection_TransceiverDirection(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	createTransceiver := func(pc *PeerConnection, dir RTPTransceiverDirection) error {
		// AddTransceiverFromKind() can't be used with sendonly
		if dir == RTPTransceiverDirectionSendonly {
			codecs := pc.api.mediaEngine.getCodecsByKind(RTPCodecTypeVideo)

			track, err := NewTrackLocalStaticSample(codecs[0].RTPCodecCapability, util.MathRandAlpha(16), util.MathRandAlpha(16))
			if err != nil {
				return err
			}

			_, err = pc.AddTransceiverFromTrack(track, []RTPTransceiverInit{
				{Direction: dir},
			}...)
			return err
		}

		_, err := pc.AddTransceiverFromKind(
			RTPCodecTypeVideo,
			RTPTransceiverInit{Direction: dir},
		)
		return err
	}

	for _, test := range []struct {
		name                  string
		offerDirection        RTPTransceiverDirection
		answerStartDirection  RTPTransceiverDirection
		answerFinalDirections []RTPTransceiverDirection
	}{
		{
			"offer sendrecv answer sendrecv",
			RTPTransceiverDirectionSendrecv,
			RTPTransceiverDirectionSendrecv,
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendrecv},
		},
		{
			"offer sendonly answer sendrecv",
			RTPTransceiverDirectionSendonly,
			RTPTransceiverDirectionSendrecv,
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendrecv},
		},
		{
			"offer recvonly answer sendrecv",
			RTPTransceiverDirectionRecvonly,
			RTPTransceiverDirectionSendrecv,
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendonly},
		},
		{
			"offer sendrecv answer sendonly",
			RTPTransceiverDirectionSendrecv,
			RTPTransceiverDirectionSendonly,
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendrecv},
		},
		{
			"offer sendonly answer sendonly",
			RTPTransceiverDirectionSendonly,
			RTPTransceiverDirectionSendonly,
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendonly, RTPTransceiverDirectionRecvonly},
		},
		{
			"offer recvonly answer sendonly",
			RTPTransceiverDirectionRecvonly,
			RTPTransceiverDirectionSendonly,
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendonly},
		},
		{
			"offer sendrecv answer recvonly",
			RTPTransceiverDirectionSendrecv,
			RTPTransceiverDirectionRecvonly,
			[]RTPTransceiverDirection{RTPTransceiverDirectionRecvonly},
		},
		{
			"offer sendonly answer recvonly",
			RTPTransceiverDirectionSendonly,
			RTPTransceiverDirectionRecvonly,
			[]RTPTransceiverDirection{RTPTransceiverDirectionRecvonly},
		},
		{
			"offer recvonly answer recvonly",
			RTPTransceiverDirectionRecvonly,
			RTPTransceiverDirectionRecvonly,
			[]RTPTransceiverDirection{RTPTransceiverDirectionRecvonly, RTPTransceiverDirectionSendonly},
		},
	} {
		offerDirection := test.offerDirection
		answerStartDirection := test.answerStartDirection
		answerFinalDirections := test.answerFinalDirections

		t.Run(test.name, func(t *testing.T) {
			pcOffer, pcAnswer, err := newPair()
			assert.NoError(t, err)

			err = createTransceiver(pcOffer, offerDirection)
			assert.NoError(t, err)

			offer, err := pcOffer.CreateOffer(nil)
			assert.NoError(t, err)

			err = createTransceiver(pcAnswer, answerStartDirection)
			assert.NoError(t, err)

			assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

			assert.Equal(t, len(answerFinalDirections), len(pcAnswer.GetTransceivers()))

			for i, tr := range pcAnswer.GetTransceivers() {
				assert.Equal(t, answerFinalDirections[i], tr.Direction())
			}

			assert.NoError(t, pcOffer.Close())
			assert.NoError(t, pcAnswer.Close())
		})
	}
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
		offer := pcOffer.LocalDescription().parsed
		sessionID := offer.Origin.SessionID
		sessionVersion := offer.Origin.SessionVersion
		if offerSessionID == 0 {
			offerSessionID = sessionID
			offerSessionVersion = sessionVersion
		} else {
			if offerSessionID != sessionID {
				t.Errorf("offer[%v] session id mismatch: expected=%v, got=%v", i, offerSessionID, sessionID)
			}
			if offerSessionVersion+1 != sessionVersion {
				t.Errorf("offer[%v] session version mismatch: expected=%v, got=%v", i, offerSessionVersion+1, sessionVersion)
			}
			offerSessionVersion++
		}

		answer := pcAnswer.LocalDescription().parsed
		sessionID = answer.Origin.SessionID
		sessionVersion = answer.Origin.SessionVersion
		if answerSessionID == 0 {
			answerSessionID = sessionID
			answerSessionVersion = sessionVersion
		} else {
			if answerSessionID != sessionID {
				t.Errorf("answer[%v] session id mismatch: expected=%v, got=%v", i, answerSessionID, sessionID)
			}
			if answerSessionVersion+1 != sessionVersion {
				t.Errorf("answer[%v] session version mismatch: expected=%v, got=%v", i, answerSessionVersion+1, sessionVersion)
			}
			answerSessionVersion++
		}
	}
	closePairNow(t, pcOffer, pcAnswer)
}

func TestPeerConnectionNilCallback(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pc.onSignalingStateChange(SignalingStateStable)
	pc.OnSignalingStateChange(func(ss SignalingState) {
		t.Error("OnSignalingStateChange called")
	})
	pc.OnSignalingStateChange(nil)
	pc.onSignalingStateChange(SignalingStateStable)

	pc.onConnectionStateChange(PeerConnectionStateNew)
	pc.OnConnectionStateChange(func(pcs PeerConnectionState) {
		t.Error("OnConnectionStateChange called")
	})
	pc.OnConnectionStateChange(nil)
	pc.onConnectionStateChange(PeerConnectionStateNew)

	pc.onICEConnectionStateChange(ICEConnectionStateNew)
	pc.OnICEConnectionStateChange(func(ics ICEConnectionState) {
		t.Error("OnConnectionStateChange called")
	})
	pc.OnICEConnectionStateChange(nil)
	pc.onICEConnectionStateChange(ICEConnectionStateNew)

	pc.onNegotiationNeeded()
	pc.negotiationNeededOp()
	pc.OnNegotiationNeeded(func() {
		t.Error("OnNegotiationNeeded called")
	})
	pc.OnNegotiationNeeded(nil)
	pc.onNegotiationNeeded()
	pc.negotiationNeededOp()

	assert.NoError(t, pc.Close())
}

func TestTransceiverCreatedByRemoteSdpHasSameCodecOrderAsRemote(t *testing.T) {
	t.Run("Codec MatchExact", func(t *testing.T) { //nolint:dupl
		const remoteSdp = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=video 60323 UDP/TLS/RTP/SAVPF 98 94 106
a=ice-ufrag:1/MvHwjAyVf27aLu
a=ice-pwd:3dBU7cFOBl120v33cynDvN1E
a=ice-options:google-ice
a=fingerprint:sha-256 75:74:5A:A6:A4:E5:52:F4:A7:67:4C:01:C7:EE:91:3F:21:3D:A2:E3:53:7B:6F:30:86:F2:30:AA:65:FB:04:24
a=mid:0
a=rtpmap:98 H264/90000
a=fmtp:98 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f
a=rtpmap:94 VP8/90000
a=rtpmap:106 H264/90000
a=fmtp:106 level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42e01f
a=sendonly
m=video 60323 UDP/TLS/RTP/SAVPF 108 98 125
a=ice-ufrag:1/MvHwjAyVf27aLu
a=ice-pwd:3dBU7cFOBl120v33cynDvN1E
a=ice-options:google-ice
a=fingerprint:sha-256 75:74:5A:A6:A4:E5:52:F4:A7:67:4C:01:C7:EE:91:3F:21:3D:A2:E3:53:7B:6F:30:86:F2:30:AA:65:FB:04:24
a=mid:1
a=rtpmap:98 H264/90000
a=fmtp:98 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f
a=rtpmap:108 VP8/90000
a=sendonly
a=rtpmap:125 H264/90000
a=fmtp:125 level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42e01f
`
		m := MediaEngine{}
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        94,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f", nil},
			PayloadType:        98,
		}, RTPCodecTypeVideo))

		api := NewAPI(WithMediaEngine(&m))
		pc, err := api.NewPeerConnection(Configuration{})
		assert.NoError(t, err)
		assert.NoError(t, pc.SetRemoteDescription(SessionDescription{
			Type: SDPTypeOffer,
			SDP:  remoteSdp,
		}))
		ans, _ := pc.CreateAnswer(nil)
		assert.NoError(t, pc.SetLocalDescription(ans))
		codecOfTr1 := pc.GetTransceivers()[0].getCodecs()[0]
		codecs := pc.api.mediaEngine.getCodecsByKind(RTPCodecTypeVideo)
		_, matchType := codecParametersFuzzySearch(codecOfTr1, codecs)
		assert.Equal(t, codecMatchExact, matchType)
		codecOfTr2 := pc.GetTransceivers()[1].getCodecs()[0]
		_, matchType = codecParametersFuzzySearch(codecOfTr2, codecs)
		assert.Equal(t, codecMatchExact, matchType)
		assert.EqualValues(t, 94, codecOfTr2.PayloadType)
		assert.NoError(t, pc.Close())
	})

	t.Run("Codec PartialExact Only", func(t *testing.T) { //nolint:dupl
		const remoteSdp = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=video 60323 UDP/TLS/RTP/SAVPF 98 106
a=ice-ufrag:1/MvHwjAyVf27aLu
a=ice-pwd:3dBU7cFOBl120v33cynDvN1E
a=ice-options:google-ice
a=fingerprint:sha-256 75:74:5A:A6:A4:E5:52:F4:A7:67:4C:01:C7:EE:91:3F:21:3D:A2:E3:53:7B:6F:30:86:F2:30:AA:65:FB:04:24
a=mid:0
a=rtpmap:98 H264/90000
a=fmtp:98 level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42e01f
a=rtpmap:106 H264/90000
a=fmtp:106 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640032
a=sendonly
m=video 60323 UDP/TLS/RTP/SAVPF 125 98
a=ice-ufrag:1/MvHwjAyVf27aLu
a=ice-pwd:3dBU7cFOBl120v33cynDvN1E
a=ice-options:google-ice
a=fingerprint:sha-256 75:74:5A:A6:A4:E5:52:F4:A7:67:4C:01:C7:EE:91:3F:21:3D:A2:E3:53:7B:6F:30:86:F2:30:AA:65:FB:04:24
a=mid:1
a=rtpmap:125 H264/90000
a=fmtp:125 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640032
a=rtpmap:98 H264/90000
a=fmtp:98 level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42e01f
a=sendonly
`
		m := MediaEngine{}
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        94,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f", nil},
			PayloadType:        98,
		}, RTPCodecTypeVideo))

		api := NewAPI(WithMediaEngine(&m))
		pc, err := api.NewPeerConnection(Configuration{})
		assert.NoError(t, err)
		assert.NoError(t, pc.SetRemoteDescription(SessionDescription{
			Type: SDPTypeOffer,
			SDP:  remoteSdp,
		}))
		ans, _ := pc.CreateAnswer(nil)
		assert.NoError(t, pc.SetLocalDescription(ans))
		codecOfTr1 := pc.GetTransceivers()[0].getCodecs()[0]
		codecs := pc.api.mediaEngine.getCodecsByKind(RTPCodecTypeVideo)
		_, matchType := codecParametersFuzzySearch(codecOfTr1, codecs)
		assert.Equal(t, codecMatchExact, matchType)
		codecOfTr2 := pc.GetTransceivers()[1].getCodecs()[0]
		_, matchType = codecParametersFuzzySearch(codecOfTr2, codecs)
		assert.Equal(t, codecMatchExact, matchType)
		// h.264/profile-id=640032 should be remap to 106 as same as transceiver 1
		assert.EqualValues(t, 106, codecOfTr2.PayloadType)
		assert.NoError(t, pc.Close())
	})
}

// Assert that remote candidates with an unknown transport are ignored and logged.
// This allows us to accept SessionDescriptions with proprietary candidates
// like `ssltcp`.
func TestInvalidCandidateTransport(t *testing.T) {
	const (
		sslTCPCandidate = `candidate:1 1 ssltcp 1 127.0.0.1 443 typ host generation 0`
		sslTCPOffer     = `v=0
o=- 0 2 IN IP4 127.0.0.1
s=-
t=0 0
a=msid-semantic: WMS
m=application 9 DTLS/SCTP 5000
c=IN IP4 0.0.0.0
a=ice-ufrag:1/MvHwjAyVf27aLu
a=ice-pwd:3dBU7cFOBl120v33cynDvN1E
a=fingerprint:sha-256 75:74:5A:A6:A4:E5:52:F4:A7:67:4C:01:C7:EE:91:3F:21:3D:A2:E3:53:7B:6F:30:86:F2:30:AA:65:FB:04:24
a=mid:0
a=` + sslTCPCandidate + "\n"
	)

	peerConnection, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, peerConnection.SetRemoteDescription(SessionDescription{Type: SDPTypeOffer, SDP: sslTCPOffer}))
	assert.NoError(t, peerConnection.AddICECandidate(ICECandidateInit{Candidate: sslTCPCandidate}))

	assert.NoError(t, peerConnection.Close())
}

func TestOfferWithInactiveDirection(t *testing.T) {
	const remoteSDP = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
a=fingerprint:sha-256 F7:BF:B4:42:5B:44:C0:B9:49:70:6D:26:D7:3E:E6:08:B1:5B:25:2E:32:88:50:B6:3C:BE:4E:18:A7:2C:85:7C
a=group:BUNDLE 0 1
a=msid-semantic:WMS *
m=video 9 UDP/TLS/RTP/SAVPF 97
c=IN IP4 0.0.0.0
a=inactive
a=ice-pwd:05d682b2902af03db90d9a9a5f2f8d7f
a=ice-ufrag:93cc7e4d
a=mid:0
a=rtpmap:97 H264/90000
a=setup:actpass
a=ssrc:1455629982 cname:{61fd3093-0326-4b12-8258-86bdc1fe677a}
`

	peerConnection, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, peerConnection.SetRemoteDescription(SessionDescription{Type: SDPTypeOffer, SDP: remoteSDP}))
	assert.Equal(t, RTPTransceiverDirectionInactive, peerConnection.rtpTransceivers[0].direction.Load().(RTPTransceiverDirection)) //nolint:forcetypeassert

	assert.NoError(t, peerConnection.Close())
}

func TestPeerConnectionState(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	assert.Equal(t, PeerConnectionStateNew, pc.ConnectionState())

	pc.updateConnectionState(ICEConnectionStateChecking, DTLSTransportStateNew)
	assert.Equal(t, PeerConnectionStateConnecting, pc.ConnectionState())

	pc.updateConnectionState(ICEConnectionStateConnected, DTLSTransportStateNew)
	assert.Equal(t, PeerConnectionStateConnecting, pc.ConnectionState())

	pc.updateConnectionState(ICEConnectionStateConnected, DTLSTransportStateConnecting)
	assert.Equal(t, PeerConnectionStateConnecting, pc.ConnectionState())

	pc.updateConnectionState(ICEConnectionStateConnected, DTLSTransportStateConnected)
	assert.Equal(t, PeerConnectionStateConnected, pc.ConnectionState())

	pc.updateConnectionState(ICEConnectionStateCompleted, DTLSTransportStateConnected)
	assert.Equal(t, PeerConnectionStateConnected, pc.ConnectionState())

	pc.updateConnectionState(ICEConnectionStateConnected, DTLSTransportStateClosed)
	assert.Equal(t, PeerConnectionStateConnected, pc.ConnectionState())

	pc.updateConnectionState(ICEConnectionStateDisconnected, DTLSTransportStateConnected)
	assert.Equal(t, PeerConnectionStateDisconnected, pc.ConnectionState())

	pc.updateConnectionState(ICEConnectionStateFailed, DTLSTransportStateConnected)
	assert.Equal(t, PeerConnectionStateFailed, pc.ConnectionState())

	pc.updateConnectionState(ICEConnectionStateConnected, DTLSTransportStateFailed)
	assert.Equal(t, PeerConnectionStateFailed, pc.ConnectionState())

	assert.NoError(t, pc.Close())
	assert.Equal(t, PeerConnectionStateClosed, pc.ConnectionState())
}
