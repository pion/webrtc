// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
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
	"net"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/dtls/v3"
	"github.com/pion/ice/v4"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/pion/srtp/v3"
	"github.com/pion/transport/v4/test"
	"github.com/pion/transport/v4/vnet"
	"github.com/pion/turn/v4"
	"github.com/pion/webrtc/v4/internal/util"
	"github.com/pion/webrtc/v4/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func waitForSRTPReady(t *testing.T, pc *PeerConnection) {
	t.Helper()

	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()

	select {
	case <-pc.dtlsTransport.srtpReady:
		return
	case <-timer.C:
		assert.FailNow(t, "timed out waiting for SRTP startup")
	}
}

func hasCryptexForMedia(t *testing.T, sdpStr string, mediaKind string) bool {
	t.Helper()

	parsed := &sdp.SessionDescription{}
	require.NoError(t, parsed.Unmarshal([]byte(sdpStr)))

	for _, md := range parsed.MediaDescriptions {
		if md.MediaName.Media != mediaKind {
			continue
		}

		return isCryptexSet(md)
	}

	assert.FailNowf(t, "media kind %s not found in SDP", mediaKind)

	return false
}

func hasCryptexForSession(t *testing.T, sdpStr string) bool {
	t.Helper()

	parsed := &sdp.SessionDescription{}
	require.NoError(t, parsed.Unmarshal([]byte(sdpStr)))

	if _, hasCryptex := parsed.Attribute(sdp.AttrKeyCryptex); hasCryptex {
		return true
	}

	return false
}

func dtlsCryptexMode(pc *PeerConnection) srtp.CryptexMode {
	pc.dtlsTransport.lock.RLock()
	defer pc.dtlsTransport.lock.RUnlock()

	return pc.dtlsTransport.cryptexMode
}

func findTransceiverByKind(pc *PeerConnection, kind RTPCodecType) *RTPTransceiver {
	for _, tr := range pc.GetTransceivers() {
		if tr != nil && tr.Kind() == kind {
			return tr
		}
	}

	return nil
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
			ICECandidatePoolSize: 1,
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
					ICECandidatePoolSize: 1,
				})
				if err != nil {
					return pc, err
				}

				if pc.iceGatherer.validatedServersCount() != 0 {
					return pc, fmt.Errorf("%w: expected 0 validated servers", ErrUnknownType)
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
					ICECandidatePoolSize: 1,
				})
				if err != nil {
					return pc, err
				}

				// Verify ICE gatherer received the new servers.
				if pc.iceGatherer.validatedServersCount() != 2 {
					return pc, fmt.Errorf("%w: expected 2 validated servers", ErrUnknownType)
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
		{
			name: "update RTPHeaderEncryptionPolicy",
			init: func() (*PeerConnection, error) {
				return NewPeerConnection(Configuration{})
			},
			config: Configuration{
				RTPHeaderEncryptionPolicy: RTPHeaderEncryptionPolicyRequire,
			},
			wantErr: &rtcerr.InvalidModificationError{Err: errModifyingRTPHeaderEncryptionPolicy},
		},
	} {
		pc, err := test.init()
		assert.NoErrorf(t, err, "SetConfiguration %q: init failed", test.name)

		err = pc.SetConfiguration(test.config)
		// This is supposed to be assert.Equal, and not assert.ErrorIs,
		// The error is a pointer to a struct.
		assert.Equal(t, test.wantErr, err, "SetConfiguration %q", test.name)

		assert.NoError(t, pc.Close())
	}
}

func TestPeerConnection_GetConfiguration_Go(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	cfg := pc.GetConfiguration()
	assert.Equal(t, false, cfg.AlwaysNegotiateDataChannels)
	assert.Equal(t, RTPHeaderEncryptionPolicyNegotiate, cfg.RTPHeaderEncryptionPolicy)

	assert.NoError(t, pc.Close())
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

	pc.OnTrack(func(*TrackRemote, *RTPReceiver) {
		close(onTrackCalled)
	})

	pc.OnICEConnectionStateChange(func(ICEConnectionState) {
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
// We ensure that DTLS is in flight by removing the mux func for it, so all inbound DTLS is lost.
func TestPeerConnection_ShutdownNoDTLS(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	offerPC, answerPC, err := api.newPair(Configuration{})
	assert.NoError(t, err)

	// Drop all incoming DTLS traffic
	dropAllDTLS := func([]byte) bool {
		return false
	}
	offerPC.dtlsTransport.dtlsMatcher = dropAllDTLS
	answerPC.dtlsTransport.dtlsMatcher = dropAllDTLS

	assert.NoError(t, signalPair(offerPC, answerPC))

	iceComplete := make(chan any)
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
	assert.NoError(t, err)
	_, err = pc.CreateAnswer(nil)
	assert.Equal(t, &rtcerr.InvalidStateError{Err: ErrNoRemoteDescription}, err)

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
			[]RTPTransceiverDirection{
				RTPTransceiverDirectionSendrecv,
				RTPTransceiverDirectionRecvonly,
				RTPTransceiverDirectionSendonly,
				RTPTransceiverDirectionInactive,
			},

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
		assert.Len(t, test.kinds, len(test.directions), "Kinds and Directions must be the same length")
		got := []*RTPTransceiver{}
		for i := range test.kinds {
			res, filteredLocalTransceivers := satisfyTypeAndDirection(test.kinds[i], test.directions[i], test.localTransceivers)

			got = append(got, res)
			test.localTransceivers = filteredLocalTransceivers
		}

		assert.Equal(t, test.want, got, "satisfyTypeAndDirection %q", test.name)
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
		assert.NoError(t, err)

		answerSettingEngine := SettingEngine{}
		answerSettingEngine.SetLite(answerisLite)
		answerPC, err := NewAPI(WithSettingEngine(answerSettingEngine)).NewPeerConnection(Configuration{})
		assert.NoError(t, err)
		assert.NoError(t, signalPair(offerPC, answerPC))

		dataChannelOpen := make(chan any)
		answerPC.OnDataChannel(func(_ *DataChannel) {
			close(dataChannelOpen)
		})

		<-dataChannelOpen
		closePairNow(t, offerPC, answerPC)
	}

	t.Run("Offerer", func(*testing.T) {
		connectTwoAgents(true, false)
	})

	t.Run("Answerer", func(*testing.T) {
		connectTwoAgents(false, true)
	})

	t.Run("Both", func(*testing.T) {
		connectTwoAgents(true, true)
	})
}

func TestOnICEGatheringStateChange(t *testing.T) {
	seenGathering := &atomic.Bool{}
	seenComplete := &atomic.Bool{}

	seenGatheringAndComplete := make(chan any)

	peerConn, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	var onStateChange func(s ICEGatheringState)
	onStateChange = func(s ICEGatheringState) {
		// Access to ICEGatherer in the callback must not cause dead lock.
		peerConn.OnICEGatheringStateChange(onStateChange)

		switch s { // nolint:exhaustive
		case ICEGatheringStateGathering:
			assert.False(t, seenGathering.Load(), "Completed before gathering")
			seenGathering.Store(true)
		case ICEGatheringStateComplete:
			seenComplete.Store(true)
		}

		if seenGathering.Load() && seenComplete.Load() {
			close(seenGatheringAndComplete)
		}
	}
	peerConn.OnICEGatheringStateChange(onStateChange)

	offer, err := peerConn.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, peerConn.SetLocalDescription(offer))

	select {
	case <-time.After(time.Second * 10):
		assert.Fail(t, "Gathering and Complete were never seen")
	case <-seenGatheringAndComplete:
	}

	assert.NoError(t, peerConn.Close())
}

// Assert Trickle ICE behaviors.
func TestPeerConnectionTrickle(t *testing.T) { //nolint:cyclop
	offerPC, answerPC, err := newPair()
	assert.NoError(t, err)

	_, err = offerPC.CreateDataChannel("test-channel", nil)
	assert.NoError(t, err)

	addOrCacheCandidate := func(
		pc *PeerConnection,
		c *ICECandidate,
		candidateCache []ICECandidateInit,
	) []ICECandidateInit {
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
		assert.False(t, offerCandidateDone, "Received OnICECandidate after finishing gathering")
		if c == nil {
			offerCandidateDone = true
		}

		candidateLock.Lock()
		defer candidateLock.Unlock()

		cachedOfferCandidates = addOrCacheCandidate(answerPC, c, cachedOfferCandidates)
	})

	cachedAnswerCandidates := []ICECandidateInit{}
	answerPC.OnICECandidate(func(c *ICECandidate) {
		assert.False(t, answerCandidateDone, "Received OnICECandidate after finishing gathering")
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

// Issue #1121, assert populateLocalCandidates doesn't mutate.
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

// Assert that two agents that only generate mDNS candidates can connect.
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
	pcAnswer.OnDataChannel(func(*DataChannel) {
		onDataChannelCancel()
	})
	<-onDataChannel.Done()

	closePairNow(t, pcOffer, pcAnswer)
}

func TestMulticastDNSHostNameConnection(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	offerHostName := fmt.Sprintf("pion-mdns-%s.local", strings.ToLower(util.MathRandAlpha(12)))
	answerHostName := fmt.Sprintf("pion-mdns-%s.local", strings.ToLower(util.MathRandAlpha(12)))
	for offerHostName == answerHostName {
		answerHostName = fmt.Sprintf("pion-mdns-%s.local", strings.ToLower(util.MathRandAlpha(12)))
	}

	offerSettingEngine := SettingEngine{}
	offerSettingEngine.SetICEMulticastDNSMode(ice.MulticastDNSModeQueryAndGather)
	offerSettingEngine.SetMulticastDNSHostName(offerHostName)

	answerSettingEngine := SettingEngine{}
	answerSettingEngine.SetICEMulticastDNSMode(ice.MulticastDNSModeQueryAndGather)
	answerSettingEngine.SetMulticastDNSHostName(answerHostName)

	pcOffer, err := NewAPI(WithSettingEngine(offerSettingEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	if err != nil {
		return
	}

	pcAnswer, err := NewAPI(WithSettingEngine(answerSettingEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	if err != nil {
		return
	}
	defer closePairNow(t, pcOffer, pcAnswer)

	connected := untilConnectionState(PeerConnectionStateConnected, pcOffer, pcAnswer)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))
	connected.Wait()

	offerLocal := pcOffer.LocalDescription()
	assert.NotNil(t, offerLocal)
	if offerLocal != nil {
		assert.Contains(t, offerLocal.SDP, offerHostName)
	}

	answerLocal := pcAnswer.LocalDescription()
	assert.NotNil(t, answerLocal)
	if answerLocal != nil {
		assert.Contains(t, answerLocal.SDP, answerHostName)
	}

	offerRemote := pcOffer.RemoteDescription()
	assert.NotNil(t, offerRemote)
	if offerRemote != nil {
		assert.Contains(t, offerRemote.SDP, answerHostName)
	}

	answerRemote := pcAnswer.RemoteDescription()
	assert.NotNil(t, answerRemote)
	if answerRemote != nil {
		assert.Contains(t, answerRemote.SDP, offerHostName)
	}
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

// Assert error handling when an Agent is restart.
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

	offerPeerConnection, answerPeerConnection, wan := createVNetPair(t, nil)

	pushICEState := func(i ICEConnectionState) { iceStates <- i }
	offerPeerConnection.OnICEConnectionStateChange(pushICEState)
	answerPeerConnection.OnICEConnectionStateChange(pushICEState)

	keepPackets := &atomic.Bool{}
	keepPackets.Store(true)

	// Add a filter that monitors the traffic on the router
	wan.AddChunkFilter(func(vnet.Chunk) bool {
		return keepPackets.Load()
	})

	const testMessage = "testMessage"

	d, err := answerPeerConnection.CreateDataChannel("foo", nil)
	assert.NoError(t, err)

	dataChannelMessages := make(chan string, 100)
	d.OnMessage(func(m DataChannelMessage) {
		dataChannelMessages <- string(m.Data)
	})

	dataChannelAnswerer := make(chan *DataChannel)
	offerPeerConnection.OnDataChannel(func(dataChannel *DataChannel) {
		dataChannel.OnOpen(func() {
			dataChannelAnswerer <- dataChannel
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
	keepPackets.Store(false)
	blockUntilICEState(ICEConnectionStateFailed)
	assert.NoError(t, dataChannel.SendText(testMessage))

	// ICE Restart and assert we have reconnected
	// block until our DataChannel message is delivered
	keepPackets.Store(true)
	connectWithICERestart(offerPeerConnection, answerPeerConnection)
	blockUntilICEState(ICEConnectionStateConnected)
	assert.Equal(t, testMessage, <-dataChannelMessages)

	assert.NoError(t, wan.Stop())
	closePairNow(t, offerPeerConnection, answerPeerConnection)
}

func TestPeerConnection_ICERestart_SetConfiguration_NewServers(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	// Set up vnet with a STUN server.
	const (
		offerIP  = "1.2.3.4"
		answerIP = "1.2.3.5"
		stunIP   = "1.2.3.100"
		stunPort = 3478
	)

	loggerFactory := logging.NewDefaultLoggerFactory()

	wan, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "1.2.3.0/24",
		LoggerFactory: loggerFactory,
	})
	assert.NoError(t, err)

	offerNet, err := vnet.NewNet(&vnet.NetConfig{StaticIPs: []string{offerIP}})
	assert.NoError(t, err)
	answerNet, err := vnet.NewNet(&vnet.NetConfig{StaticIPs: []string{answerIP}})
	assert.NoError(t, err)
	stunNet, err := vnet.NewNet(&vnet.NetConfig{StaticIPs: []string{stunIP}})
	assert.NoError(t, err)

	assert.NoError(t, wan.AddNet(offerNet))
	assert.NoError(t, wan.AddNet(answerNet))
	assert.NoError(t, wan.AddNet(stunNet))
	assert.NoError(t, wan.Start())

	// Create STUN server.
	stunListener, err := stunNet.ListenPacket("udp4", fmt.Sprintf("%s:%d", stunIP, stunPort))
	assert.NoError(t, err)
	stunServer, err := turn.NewServer(turn.ServerConfig{
		Realm:         "pion.ly",
		LoggerFactory: loggerFactory,
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: stunListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(stunIP),
					Address:      "0.0.0.0",
					Net:          stunNet,
				},
			},
		},
	})
	assert.NoError(t, err)

	// Create peer connections.
	offerSettingEngine := SettingEngine{}
	offerSettingEngine.SetNet(offerNet)
	offerSettingEngine.SetICETimeouts(time.Second, time.Second, time.Millisecond*200)

	answerSettingEngine := SettingEngine{}
	answerSettingEngine.SetNet(answerNet)
	answerSettingEngine.SetICETimeouts(time.Second, time.Second, time.Millisecond*200)

	offerPC, err := NewAPI(WithSettingEngine(offerSettingEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	answerPC, err := NewAPI(WithSettingEngine(answerSettingEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = offerPC.CreateDataChannel("test", nil)
	assert.NoError(t, err)

	// Initial negotiation without STUN servers.
	offer, err := offerPC.CreateOffer(nil)
	assert.NoError(t, err)

	offerGatherComplete := GatheringCompletePromise(offerPC)
	assert.NoError(t, offerPC.SetLocalDescription(offer))
	<-offerGatherComplete

	// Verify initial offer has no srflx candidates.
	assert.NotContains(t, offerPC.LocalDescription().SDP, "srflx",
		"should not have srflx candidates without STUN servers")

	assert.NoError(t, answerPC.SetRemoteDescription(*offerPC.LocalDescription()))

	answer, err := answerPC.CreateAnswer(nil)
	assert.NoError(t, err)

	answerGatherComplete := GatheringCompletePromise(answerPC)
	assert.NoError(t, answerPC.SetLocalDescription(answer))
	<-answerGatherComplete

	assert.NoError(t, offerPC.SetRemoteDescription(*answerPC.LocalDescription()))

	// Update configuration with local STUN server.
	stunURL := fmt.Sprintf("stun:%s:%d", stunIP, stunPort)
	newConfig := Configuration{
		ICEServers: []ICEServer{
			{URLs: []string{stunURL}},
		},
	}
	assert.Equal(t, 0, offerPC.iceGatherer.validatedServersCount())
	err = offerPC.SetConfiguration(newConfig)
	assert.NoError(t, err)
	assert.Equal(t, 1, offerPC.iceGatherer.validatedServersCount())

	// Trigger ICE restart.
	offer, err = offerPC.CreateOffer(&OfferOptions{ICERestart: true})
	assert.NoError(t, err)

	offerGatherComplete = GatheringCompletePromise(offerPC)
	assert.NoError(t, offerPC.SetLocalDescription(offer))
	<-offerGatherComplete

	// Verify the offer now has srflx candidates from the STUN server.
	assert.Contains(t, offerPC.LocalDescription().SDP, "srflx",
		"should have srflx candidates after restart with STUN servers")

	assert.NoError(t, answerPC.SetRemoteDescription(*offerPC.LocalDescription()))

	answer, err = answerPC.CreateAnswer(nil)
	assert.NoError(t, err)

	answerGatherComplete = GatheringCompletePromise(answerPC)
	assert.NoError(t, answerPC.SetLocalDescription(answer))
	<-answerGatherComplete

	assert.NoError(t, offerPC.SetRemoteDescription(*answerPC.LocalDescription()))

	assert.NoError(t, stunServer.Close())
	assert.NoError(t, wan.Stop())
	closePairNow(t, offerPC, answerPC)
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
func TestPeerConnection_MassiveTracks(t *testing.T) { //nolint:cyclop
	var (
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
	offerPC, answerPC, err := newPair()
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
			assert.Fail(t, "unable to receive all track events in time")
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
		assert.NoErrorf(t, err, "Case %d failed", i)

		err = peerConn.SetRemoteDescription(SessionDescription{Type: SDPTypeOffer, SDP: minimalOffer})
		assert.NoErrorf(t, err, "Case %d failed", i)

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
// pion will take the ICE-CONTROLLING role.
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
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendrecv, RTPTransceiverDirectionRecvonly},
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

func TestPeerConnection_MediaDirectionInSDP(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	createTransceiver := func(pc *PeerConnection, dir RTPTransceiverDirection) (*RTPSender, error) {
		// AddTransceiverFromKind() can't be used with sendonly
		if dir == RTPTransceiverDirectionSendonly {
			codecs := pc.api.mediaEngine.getCodecsByKind(RTPCodecTypeVideo)

			track, err := NewTrackLocalStaticSample(codecs[0].RTPCodecCapability, util.MathRandAlpha(16), util.MathRandAlpha(16))
			if err != nil {
				return nil, err
			}

			transceiver, err := pc.AddTransceiverFromTrack(track, []RTPTransceiverInit{
				{Direction: dir},
			}...)

			return transceiver.Sender(), err
		}

		transceiver, err := pc.AddTransceiverFromKind(
			RTPCodecTypeVideo,
			RTPTransceiverInit{Direction: dir},
		)

		return transceiver.Sender(), err
	}

	testCases := []struct {
		remoteDirections         []RTPTransceiverDirection
		numExpectedTransceivers  int
		numExpectedMediaSections int
		localDirections          []RTPTransceiverDirection
	}{
		{
			remoteDirections: []RTPTransceiverDirection{
				RTPTransceiverDirectionSendonly,
				RTPTransceiverDirectionInactive,
			},
			numExpectedTransceivers:  2,
			numExpectedMediaSections: 1,
			localDirections: []RTPTransceiverDirection{
				RTPTransceiverDirectionRecvonly,
				RTPTransceiverDirectionInactive,
			},
		},
		{
			remoteDirections: []RTPTransceiverDirection{
				RTPTransceiverDirectionSendrecv,
				RTPTransceiverDirectionRecvonly,
			},
			numExpectedTransceivers:  1,
			numExpectedMediaSections: 1,
			localDirections: []RTPTransceiverDirection{
				RTPTransceiverDirectionSendrecv,
				RTPTransceiverDirectionSendonly,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run("add track before remote description - "+testCase.remoteDirections[0].String(), func(t *testing.T) {
			pcOffer, pcAnswer, err := newPair()
			assert.NoError(t, err)

			// add track to answerer before any remote description, added transceiver will be `sendrecv`
			track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo", "bar")
			assert.NoError(t, err)
			_, err = pcAnswer.AddTrack(track)
			assert.NoError(t, err)

			sender, err := createTransceiver(pcOffer, testCase.remoteDirections[0])
			assert.NoError(t, err)

			offer, err := pcOffer.CreateOffer(nil)
			assert.NoError(t, err)
			assert.NoError(t, pcOffer.SetLocalDescription(offer))

			// transceiver created from remote description
			//  - cannot match track added above if remote direction is `sendonly`
			//  - can match track added above if remote direction is `sendrecv`
			assert.NoError(t, pcAnswer.SetRemoteDescription(offer))
			assert.Equal(t, testCase.numExpectedTransceivers, len(pcAnswer.GetTransceivers()))

			answer, err := pcAnswer.CreateAnswer(nil)
			assert.NoError(t, err)

			// direction has to be `recvonly` in answer if remote direction is `sendonly`
			// direction has to be `sendrecv` in answer if remote direction is `sendrecv`
			parsed, err := answer.Unmarshal()
			assert.NoError(t, err)
			assert.Equal(t, testCase.numExpectedMediaSections, len(parsed.MediaDescriptions))
			_, ok := parsed.MediaDescriptions[0].Attribute(testCase.localDirections[0].String())
			assert.True(t, ok)

			assert.NoError(t, pcAnswer.SetLocalDescription(answer))
			assert.NoError(t, pcOffer.SetRemoteDescription(answer))

			// remove the remote track and re-negotiate
			//  - both directions should become `inactive` if original remote direction was `sendonly`
			//  - remote direction should become `recvonly and local direction should become `sendonly`
			//    if original remote direction was `sendrecv`
			assert.NoError(t, pcOffer.RemoveTrack(sender))

			offer, err = pcOffer.CreateOffer(nil)
			assert.NoError(t, err)

			// offer direction should have changed to the following after removing track
			//   - `inactive` if original offer direction was `sendonly`
			//   - `recvonly` if original offer direction was `sendrecv`
			parsed, err = offer.Unmarshal()
			assert.NoError(t, err)
			assert.Equal(t, testCase.numExpectedMediaSections, len(parsed.MediaDescriptions))
			_, ok = parsed.MediaDescriptions[0].Attribute(testCase.remoteDirections[1].String())
			assert.True(t, ok)

			assert.NoError(t, pcOffer.SetLocalDescription(offer))
			assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

			answer, err = pcAnswer.CreateAnswer(nil)
			assert.NoError(t, err)

			// answer direction should have changed to
			//   - `inactive` if original offer direction was `sendonly`
			//   - `sendonly` if original offer direction was `sendrecv`
			parsed, err = answer.Unmarshal()
			assert.NoError(t, err)
			assert.Equal(t, testCase.numExpectedMediaSections, len(parsed.MediaDescriptions))
			_, ok = parsed.MediaDescriptions[0].Attribute(testCase.localDirections[1].String())
			assert.True(t, ok)

			closePairNow(t, pcOffer, pcAnswer)
		})
	}
}

func TestPeerConnectionNilCallback(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pc.onSignalingStateChange(SignalingStateStable)
	pc.OnSignalingStateChange(func(SignalingState) {
		assert.Fail(t, "OnSignalingStateChange called")
	})
	pc.OnSignalingStateChange(nil)
	pc.onSignalingStateChange(SignalingStateStable)

	pc.onConnectionStateChange(PeerConnectionStateNew)
	pc.OnConnectionStateChange(func(PeerConnectionState) {
		assert.Fail(t, "OnConnectionStateChange called")
	})
	pc.OnConnectionStateChange(nil)
	pc.onConnectionStateChange(PeerConnectionStateNew)

	pc.onICEConnectionStateChange(ICEConnectionStateNew)
	pc.OnICEConnectionStateChange(func(ICEConnectionState) {
		assert.Fail(t, "OnICEConnectionStateChange called")
	})
	pc.OnICEConnectionStateChange(nil)
	pc.onICEConnectionStateChange(ICEConnectionStateNew)

	pc.onNegotiationNeeded()
	pc.negotiationNeededOp()
	pc.OnNegotiationNeeded(func() {
		assert.Fail(t, "OnNegotiationNeeded called")
	})
	pc.OnNegotiationNeeded(nil)
	pc.onNegotiationNeeded()
	pc.negotiationNeededOp()

	assert.NoError(t, pc.Close())
}

func TestTransceiverCreatedByRemoteSdpHasSameCodecOrderAsRemote(t *testing.T) {
	t.Run("Codec MatchExact and MatchPartial", func(t *testing.T) { //nolint:dupl
		const remoteSdp = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE 0 1
m=video 60323 UDP/TLS/RTP/SAVPF 98 94 106 49
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
a=rtpmap:49 H265/90000
a=fmtp:49 level-id=186;profile-id=1;tier-flag=0;tx-mode=SRST
a=sendonly
m=video 60323 UDP/TLS/RTP/SAVPF 49 108 98 125
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
a=rtpmap:49 H265/90000
a=fmtp:49 level-id=93;profile-id=1;tier-flag=0;tx-mode=SRST
`
		mediaEngine := MediaEngine{}
		assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        94,
		}, RTPCodecTypeVideo))
		assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{
				MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f", nil,
			},
			PayloadType: 98,
		}, RTPCodecTypeVideo))
		assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{
				MimeTypeH265, 90000, 0, "level-id=186;profile-id=1;tier-flag=0;tx-mode=SRST", nil,
			},
			PayloadType: 49,
		}, RTPCodecTypeVideo))

		api := NewAPI(WithMediaEngine(&mediaEngine))
		pc, err := api.NewPeerConnection(Configuration{})
		assert.NoError(t, err)
		assert.NoError(t, pc.SetRemoteDescription(SessionDescription{
			Type: SDPTypeOffer,
			SDP:  remoteSdp,
		}))

		ans, _ := pc.CreateAnswer(nil)
		assert.NoError(t, pc.SetLocalDescription(ans))

		codecs := pc.api.mediaEngine.getCodecsByKind(RTPCodecTypeVideo)

		codecsOfTr1 := pc.GetTransceivers()[0].getCodecs()
		_, matchType := codecParametersFuzzySearch(codecsOfTr1[0], codecs)
		assert.Equal(t, codecMatchExact, matchType)
		assert.EqualValues(t, 98, codecsOfTr1[0].PayloadType)
		_, matchType = codecParametersFuzzySearch(codecsOfTr1[1], codecs)
		assert.Equal(t, codecMatchExact, matchType)
		assert.EqualValues(t, 94, codecsOfTr1[1].PayloadType)
		_, matchType = codecParametersFuzzySearch(codecsOfTr1[2], codecs)
		assert.Equal(t, codecMatchExact, matchType)
		assert.EqualValues(t, 49, codecsOfTr1[2].PayloadType)

		codecsOfTr2 := pc.GetTransceivers()[1].getCodecs()
		_, matchType = codecParametersFuzzySearch(codecsOfTr2[0], codecs)
		assert.Equal(t, codecMatchExact, matchType)
		assert.EqualValues(t, 94, codecsOfTr2[0].PayloadType)
		_, matchType = codecParametersFuzzySearch(codecsOfTr2[1], codecs)
		assert.Equal(t, codecMatchExact, matchType)
		assert.EqualValues(t, 98, codecsOfTr2[1].PayloadType)
		// as H.265 (49) is a partial match, it gets pushed to the end
		_, matchType = codecParametersFuzzySearch(codecsOfTr2[2], codecs)
		assert.Equal(t, codecMatchPartial, matchType)
		assert.EqualValues(t, 49, codecsOfTr2[2].PayloadType)

		assert.NoError(t, pc.Close())
	})

	t.Run("Codec PartialExact Only", func(t *testing.T) { //nolint:dupl
		const remoteSdp = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE 0 1
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
		mediaEngine := MediaEngine{}
		assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        94,
		}, RTPCodecTypeVideo))
		assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{
				MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f", nil,
			},
			PayloadType: 98,
		}, RTPCodecTypeVideo))

		api := NewAPI(WithMediaEngine(&mediaEngine))
		pc, err := api.NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		assert.NoError(t, pc.SetRemoteDescription(SessionDescription{
			Type: SDPTypeOffer,
			SDP:  remoteSdp,
		}))

		ans, _ := pc.CreateAnswer(nil)
		assert.NoError(t, pc.SetLocalDescription(ans))

		codecs := pc.api.mediaEngine.getCodecsByKind(RTPCodecTypeVideo)

		codecsOfTr1 := pc.GetTransceivers()[0].getCodecs()
		_, matchType := codecParametersFuzzySearch(codecsOfTr1[0], codecs)
		assert.Equal(t, codecMatchExact, matchType)
		assert.EqualValues(t, 98, codecsOfTr1[0].PayloadType)
		_, matchType = codecParametersFuzzySearch(codecsOfTr1[1], codecs)
		assert.Equal(t, codecMatchExact, matchType)
		assert.EqualValues(t, 106, codecsOfTr1[1].PayloadType)

		codecsOfTr2 := pc.GetTransceivers()[1].getCodecs()
		_, matchType = codecParametersFuzzySearch(codecsOfTr2[0], codecs)
		assert.Equal(t, codecMatchExact, matchType)
		// h.264/profile-id=640032 should be remap to 106 as same as transceiver 1
		assert.EqualValues(t, 106, codecsOfTr2[0].PayloadType)
		_, matchType = codecParametersFuzzySearch(codecsOfTr2[1], codecs)
		assert.Equal(t, codecMatchExact, matchType)
		assert.EqualValues(t, 98, codecsOfTr2[1].PayloadType)

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
a=group:BUNDLE 0
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
	assert.Equal(
		t, RTPTransceiverDirectionInactive,
		peerConnection.rtpTransceivers[0].direction.Load().(RTPTransceiverDirection), //nolint:forcetypeassert
	)

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

func TestPeerConnectionDeadlock(t *testing.T) {
	lim := test.TimeOut(time.Second * 5)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	closeHdlr := func(peerConnection *PeerConnection) {
		peerConnection.OnICEConnectionStateChange(func(i ICEConnectionState) {
			if i == ICEConnectionStateFailed || i == ICEConnectionStateClosed {
				if err := peerConnection.Close(); err != nil {
					assert.NoError(t, err)
				}
			}
		})
	}

	pcOffer, pcAnswer, err := NewAPI().newPair(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	onDataChannel, onDataChannelCancel := context.WithCancel(context.Background())
	pcAnswer.OnDataChannel(func(*DataChannel) {
		onDataChannelCancel()
	})
	<-onDataChannel.Done()

	closeHdlr(pcOffer)
	closeHdlr(pcAnswer)

	closePairNow(t, pcOffer, pcAnswer)
}

// Assert that by default NULL Ciphers aren't enabled. Even if
// the remote Peer Requests a NULL Cipher we should fail.
func TestPeerConnectionNoNULLCipherDefault(t *testing.T) {
	settingEngine := SettingEngine{}
	settingEngine.SetSRTPProtectionProfiles(dtls.SRTP_NULL_HMAC_SHA1_80, dtls.SRTP_NULL_HMAC_SHA1_32)
	offerPC, err := NewAPI(WithSettingEngine(settingEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	answerPC, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, signalPair(offerPC, answerPC))

	peerConnectionClosed := make(chan struct{})
	var closeOnce sync.Once

	answerPC.OnConnectionStateChange(func(s PeerConnectionState) {
		if s == PeerConnectionStateClosed {
			closeOnce.Do(func() { close(peerConnectionClosed) })
		}
	})

	<-peerConnectionClosed
	closePairNow(t, offerPC, answerPC)
}

func TestICETricklingSupported(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	offer, err := pc.CreateOffer(&OfferOptions{
		OfferAnswerOptions: OfferAnswerOptions{ICETricklingSupported: true},
	})
	assert.NoError(t, err)
	assert.Contains(t, offer.SDP, "a=ice-options:trickle")
	assert.NoError(t, pc.Close())

	pcAnswer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	offerSDP := strings.Join([]string{
		"v=0",
		"o=- 0 0 IN IP4 127.0.0.1",
		"s=-",
		"t=0 0",
		"a=group:BUNDLE 0",
		"a=ice-ufrag:someufrag",
		"a=ice-pwd:somepwd",
		"a=fingerprint:sha-256 " +
			"F7:BF:B4:42:5B:44:C0:B9:49:70:6D:26:D7:3E:E6:08:B1:5B:25:2E:32:88:50:B6:3C:BE:4E:18:A7:2C:85:7C",
		"a=msid-semantic: WMS *",
		"m=audio 9 UDP/TLS/RTP/SAVPF 111",
		"c=IN IP4 0.0.0.0",
		"a=rtcp:9 IN IP4 0.0.0.0",
		"a=mid:0",
		"a=rtcp-mux",
		"a=setup:actpass",
		"a=rtpmap:111 opus/48000/2",
		"",
	}, "\r\n")

	assert.NoError(t, pcAnswer.SetRemoteDescription(SessionDescription{
		Type: SDPTypeOffer,
		SDP:  offerSDP,
	}))

	answer, err := pcAnswer.CreateAnswer(&AnswerOptions{
		OfferAnswerOptions: OfferAnswerOptions{ICETricklingSupported: true},
	})
	assert.NoError(t, err)
	assert.Contains(t, answer.SDP, "a=ice-options:trickle")

	assert.NoError(t, pcAnswer.Close())
}

func TestICERenominationAdvertised(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	offerSE := SettingEngine{}
	assert.NoError(t, offerSE.SetICERenomination())

	api := NewAPI(WithSettingEngine(offerSE))
	pc, err := api.NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	offer, err := pc.CreateOffer(nil)
	assert.NoError(t, err)
	assert.Contains(t, offer.SDP, "a=ice-options:renomination")
	assert.NoError(t, pc.Close())

	answerSE := SettingEngine{}
	assert.NoError(t, answerSE.SetICERenomination())

	apiAnswer := NewAPI(WithSettingEngine(answerSE))
	pcAnswer, err := apiAnswer.NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	offerSDP := strings.Join([]string{
		"v=0",
		"o=- 0 0 IN IP4 127.0.0.1",
		"s=-",
		"t=0 0",
		"a=group:BUNDLE 0",
		"a=ice-ufrag:someufrag",
		"a=ice-pwd:somepwd",
		"a=fingerprint:sha-256 " +
			"F7:BF:B4:42:5B:44:C0:B9:49:70:6D:26:D7:3E:E6:08:B1:5B:25:2E:32:88:50:B6:3C:BE:4E:18:A7:2C:85:7C",
		"a=msid-semantic: WMS *",
		"m=audio 9 UDP/TLS/RTP/SAVPF 111",
		"c=IN IP4 0.0.0.0",
		"a=rtcp:9 IN IP4 0.0.0.0",
		"a=mid:0",
		"a=rtcp-mux",
		"a=setup:actpass",
		"a=rtpmap:111 opus/48000/2",
		"",
	}, "\r\n")

	assert.NoError(t, pcAnswer.SetRemoteDescription(SessionDescription{
		Type: SDPTypeOffer,
		SDP:  offerSDP,
	}))

	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)
	assert.Contains(t, answer.SDP, "a=ice-options:renomination")

	assert.NoError(t, pcAnswer.Close())
}

// https://github.com/pion/webrtc/issues/2690
func TestPeerConnectionTrickleMediaStreamIdentification(t *testing.T) {
	const remoteSdp = `v=0
o=- 1735985477255306 1 IN IP4 127.0.0.1
s=VideoRoom 1234
t=0 0
a=group:BUNDLE 0 1
a=ice-options:trickle
a=fingerprint:sha-256 61:BF:17:29:C0:EF:B2:77:75:79:64:F9:D8:D0:03:6C:5A:D3:9A:BC:E5:F4:5A:05:4C:3C:3B:A0:B4:2B:CF:A8
a=extmap-allow-mixed
a=msid-semantic: WMS *
m=audio 9 UDP/TLS/RTP/SAVPF 111
c=IN IP4 127.0.0.1
a=sendonly
a=mid:0
a=rtcp-mux
a=ice-ufrag:xv3r
a=ice-pwd:NT22yM6JeOsahq00U9ZJS/
a=ice-options:trickle
a=setup:actpass
a=rtpmap:111 opus/48000/2
a=rtcp-fb:111 transport-cc
a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level
a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid
a=fmtp:111 useinbandfec=1
a=msid:janus janus0
a=ssrc:2280306597 cname:janus
m=video 9 UDP/TLS/RTP/SAVPF 96 97
c=IN IP4 127.0.0.1
a=sendonly
a=mid:1
a=rtcp-mux
a=ice-ufrag:xv3r
a=ice-pwd:NT22yM6JeOsahq00U9ZJS/
a=ice-options:trickle
a=setup:actpass
a=rtpmap:96 VP8/90000
a=rtcp-fb:96 ccm fir
a=rtcp-fb:96 nack
a=rtcp-fb:96 nack pli
a=rtcp-fb:96 goog-remb
a=rtcp-fb:96 transport-cc
a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time
a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01
a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid
a=extmap:12 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay
a=extmap:13 urn:3gpp:video-orientation
a=rtpmap:97 rtx/90000
a=fmtp:97 apt=96
a=ssrc-group:FID 4099488402 29586368
a=msid:janus janus1
a=ssrc:4099488402 cname:janus
a=ssrc:29586368 cname:janus
`

	mediaEngine := &MediaEngine{}

	assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType: MimeTypeVP8, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil,
		},
		PayloadType: 96,
	}, RTPCodecTypeVideo))
	assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType: MimeTypeOpus, ClockRate: 48000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil,
		},
		PayloadType: 111,
	}, RTPCodecTypeAudio))

	api := NewAPI(WithMediaEngine(mediaEngine))
	pc, err := api.NewPeerConnection(Configuration{
		ICEServers: []ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	assert.NoError(t, err)

	pc.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			return
		}

		assert.NotEmpty(t, candidate.SDPMid)

		assert.Contains(t, []string{"0", "1"}, candidate.SDPMid)
		assert.Contains(t, []uint16{0, 1}, candidate.SDPMLineIndex)
	})

	assert.NoError(t, pc.SetRemoteDescription(SessionDescription{
		Type: SDPTypeOffer,
		SDP:  remoteSdp,
	}))

	gatherComplete := GatheringCompletePromise(pc)
	ans, _ := pc.CreateAnswer(nil)
	assert.NoError(t, pc.SetLocalDescription(ans))

	<-gatherComplete

	assert.NoError(t, pc.Close())

	assert.Equal(t, PeerConnectionStateClosed, pc.ConnectionState())
}

func TestTranceiverMediaStreamIdentification(t *testing.T) {
	const videoMid = "0"
	const audioMid = "1"

	mediaEngine := &MediaEngine{}

	assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType: MimeTypeVP8, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil,
		},
		PayloadType: 96,
	}, RTPCodecTypeVideo))
	assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType: MimeTypeOpus, ClockRate: 48000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil,
		},
		PayloadType: 111,
	}, RTPCodecTypeAudio))

	api := NewAPI(WithMediaEngine(mediaEngine))
	pcOfferer, pcAnswerer, err := api.newPair(Configuration{
		ICEServers: []ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	assert.NoError(t, err)

	pcOfferer.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			return
		}

		assert.NotEmpty(t, candidate.SDPMid)
		assert.Contains(t, []string{videoMid, audioMid}, candidate.SDPMid)
		assert.Contains(t, []uint16{0, 1}, candidate.SDPMLineIndex)
	})

	pcAnswerer.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			return
		}

		assert.NotEmpty(t, candidate.SDPMid)
		assert.Contains(t, []string{videoMid, audioMid}, candidate.SDPMid)
		assert.Contains(t, []uint16{0, 1}, candidate.SDPMLineIndex)
	})

	videoTransceiver, err := pcOfferer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	})
	assert.NoError(t, err)

	audioTransceiver, err := pcOfferer.AddTransceiverFromKind(RTPCodecTypeAudio, RTPTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	})
	assert.NoError(t, err)

	assert.NoError(t, videoTransceiver.SetMid(videoMid))
	assert.NoError(t, audioTransceiver.SetMid(audioMid))

	offer, err := pcOfferer.CreateOffer(nil)
	assert.NoError(t, err)

	assert.NoError(t, pcOfferer.SetLocalDescription(offer))

	assert.NoError(t, pcAnswerer.SetRemoteDescription(offer))

	answer, err := pcAnswerer.CreateAnswer(nil)
	assert.NoError(t, err)

	assert.NoError(t, pcAnswerer.SetLocalDescription(answer))

	answerGatherComplete := GatheringCompletePromise(pcOfferer)
	offerGatherComplete := GatheringCompletePromise(pcAnswerer)

	<-answerGatherComplete
	<-offerGatherComplete

	assert.NoError(t, pcOfferer.Close())
	assert.NoError(t, pcAnswerer.Close())
}

func Test_WriteRTCP_Disconnected(t *testing.T) {
	peerConnection, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.Error(t, peerConnection.WriteRTCP(
		[]rtcp.Packet{&rtcp.RapidResynchronizationRequest{SenderSSRC: 5, MediaSSRC: 10}}),
	)

	assert.NoError(t, peerConnection.Close())
}

func Test_IPv6(t *testing.T) { //nolint: cyclop
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Skip()
	}

	IPv6Supported := false
	for _, iface := range interfaces {
		addrs, netErr := iface.Addrs()
		if netErr != nil {
			continue
		}

		// Loop over the addresses for the interface.
		for _, addr := range addrs {
			var ip net.IP

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.To4() != nil || ip.IsLinkLocalUnicast() || ip.IsLoopback() {
				continue
			}

			IPv6Supported = true
		}
	}

	if !IPv6Supported {
		t.Skip()
	}

	lim := test.TimeOut(time.Second * 5)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	settingEngine := SettingEngine{}
	settingEngine.SetNetworkTypes([]NetworkType{NetworkTypeUDP6})

	offerPC, answerPC, err := NewAPI(WithSettingEngine(settingEngine)).newPair(Configuration{})
	assert.NoError(t, err)

	peerConnectionConnected := untilConnectionState(PeerConnectionStateConnected, offerPC, answerPC)
	assert.NoError(t, signalPair(offerPC, answerPC))

	peerConnectionConnected.Wait()

	offererSelectedPair, err := offerPC.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
	assert.NoError(t, err)
	assert.NotNil(t, offererSelectedPair)

	answererSelectedPair, err := answerPC.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
	assert.NoError(t, err)
	assert.NotNil(t, answererSelectedPair)

	for _, c := range []*ICECandidate{
		answererSelectedPair.Local,
		answererSelectedPair.Remote,
		offererSelectedPair.Local,
		offererSelectedPair.Remote,
	} {
		iceCandidate, err := c.ToICE()
		assert.NoError(t, err)
		assert.Equal(t, iceCandidate.NetworkType(), ice.NetworkTypeUDP6)
	}

	closePairNow(t, offerPC, answerPC)
}

type testICELogger struct {
	lastErrorMessage string
}

func (t *testICELogger) Trace(string)          {}
func (t *testICELogger) Tracef(string, ...any) {}
func (t *testICELogger) Debug(string)          {}
func (t *testICELogger) Debugf(string, ...any) {}
func (t *testICELogger) Info(string)           {}
func (t *testICELogger) Infof(string, ...any)  {}
func (t *testICELogger) Warn(string)           {}
func (t *testICELogger) Warnf(string, ...any)  {}
func (t *testICELogger) Error(msg string)      { t.lastErrorMessage = msg }
func (t *testICELogger) Errorf(format string, args ...any) {
	t.lastErrorMessage = fmt.Sprintf(format, args...)
}

type testICELoggerFactory struct {
	logger *testICELogger
}

func (t *testICELoggerFactory) NewLogger(string) logging.LeveledLogger {
	return t.logger
}

func TestAddICECandidate__DroppingOldGenerationCandidates(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	testLogger := &testICELogger{}
	loggerFactory := &testICELoggerFactory{logger: testLogger}

	// Create a new API with the custom logger
	api := NewAPI(WithSettingEngine(SettingEngine{
		LoggerFactory: loggerFactory,
	}))

	pc, err := api.NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = pc.CreateDataChannel("test", nil)
	assert.NoError(t, err)

	offer, err := pc.CreateOffer(nil)
	assert.NoError(t, err)

	offerGatheringComplete := GatheringCompletePromise(pc)
	assert.NoError(t, pc.SetLocalDescription(offer))
	<-offerGatheringComplete

	remotePC, err := api.NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, remotePC.SetRemoteDescription(offer))

	remoteDesc := remotePC.RemoteDescription()
	assert.NotNil(t, remoteDesc)

	ufrag, hasUfrag := remoteDesc.parsed.MediaDescriptions[0].Attribute("ice-ufrag")
	assert.True(t, hasUfrag)

	emptyUfragCandidate := ICECandidateInit{
		Candidate: "candidate:1 1 UDP 2122252543 192.168.1.1 12345 typ host",
	}
	err = remotePC.AddICECandidate(emptyUfragCandidate)
	assert.NoError(t, err)
	assert.Empty(t, testLogger.lastErrorMessage)

	validCandidate := ICECandidateInit{
		Candidate: fmt.Sprintf("candidate:1 1 UDP 2122252543 192.168.1.1 12345 typ host ufrag %s", ufrag),
	}
	err = remotePC.AddICECandidate(validCandidate)
	assert.NoError(t, err)
	assert.Empty(t, testLogger.lastErrorMessage)

	invalidCandidate := ICECandidateInit{
		Candidate: "candidate:1 1 UDP 2122252543 192.168.1.1 12345 typ host ufrag invalid",
	}
	err = remotePC.AddICECandidate(invalidCandidate)
	assert.NoError(t, err)
	assert.Contains(t, testLogger.lastErrorMessage, "dropping candidate with ufrag")

	closePairNow(t, pc, remotePC)
}

func TestPeerConnectionCanTrickleICECandidatesGo(t *testing.T) {
	offerPC, answerPC, wan := createVNetPair(t, nil)
	var err error
	defer func() {
		assert.NoError(t, wan.Stop())
		closePairNow(t, offerPC, answerPC)
	}()

	_, err = offerPC.CreateDataChannel("trickle", nil)
	assert.NoError(t, err)

	offer, err := offerPC.CreateOffer(&OfferOptions{
		OfferAnswerOptions: OfferAnswerOptions{ICETricklingSupported: true},
	})
	assert.NoError(t, err)
	assert.NoError(t, offerPC.SetLocalDescription(offer))
	assert.Equal(t, ICETrickleCapabilityUnknown, answerPC.CanTrickleICECandidates())
	assert.NoError(t, answerPC.SetRemoteDescription(offer))
	assert.Equal(t, ICETrickleCapabilitySupported, answerPC.CanTrickleICECandidates())

	noTrickleOfferPC, noTrickleAnswerPC, noTrickleWAN := createVNetPair(t, nil)
	defer func() {
		assert.NoError(t, noTrickleWAN.Stop())
		closePairNow(t, noTrickleOfferPC, noTrickleAnswerPC)
	}()

	_, err = noTrickleOfferPC.CreateDataChannel("notrickle", nil)
	assert.NoError(t, err)

	noTrickleOffer, err := noTrickleOfferPC.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, noTrickleOfferPC.SetLocalDescription(noTrickleOffer))
	assert.Equal(t, ICETrickleCapabilityUnknown, noTrickleAnswerPC.CanTrickleICECandidates())
	assert.NoError(t, noTrickleAnswerPC.SetRemoteDescription(noTrickleOffer))
	assert.Equal(t, ICETrickleCapabilityUnsupported, noTrickleAnswerPC.CanTrickleICECandidates())
}

func TestCreateAnswerActiveOfferPassiveAnswer(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	activeDesc := SessionDescription{Type: SDPTypeOffer, SDP: strings.ReplaceAll(minimalOffer, "actpass", "active")}
	assert.NoError(t, pc.SetRemoteDescription(activeDesc))
	answer, err := pc.CreateAnswer(nil)
	assert.NoError(t, err)
	answerRole := dtlsRoleFromSDP(answer.parsed)
	assert.Equal(t, answerRole, DTLSRoleServer)
	assert.NoError(t, pc.Close())
}

func TestCreateAnswerPassiveOfferActiveAnswer(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	passiveDesc := SessionDescription{Type: SDPTypeOffer, SDP: strings.ReplaceAll(minimalOffer, "actpass", "passive")}
	assert.NoError(t, pc.SetRemoteDescription(passiveDesc))
	answer, err := pc.CreateAnswer(nil)
	assert.NoError(t, err)
	answerRole := dtlsRoleFromSDP(answer.parsed)
	assert.Equal(t, answerRole, DTLSRoleClient)
	assert.NoError(t, pc.Close())
}

func TestAlwaysNegotiateDataChannel_InitialOffer_Go(t *testing.T) {
	pcDefault, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	offerDefault, err := pcDefault.CreateOffer(nil)
	assert.NoError(t, err)
	assert.Nil(t, haveDataChannel(&offerDefault))
	assert.NoError(t, pcDefault.Close())

	pc, err := NewPeerConnection(Configuration{AlwaysNegotiateDataChannels: true})
	assert.NoError(t, err)

	offer, err := pc.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NotNil(t, haveDataChannel(&offer))
	assert.NoError(t, pc.Close())
}

func TestAlwaysNegotiateDataChannels_CreateDataChannel(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	cfg := Configuration{AlwaysNegotiateDataChannels: true}

	pcOffer, err := NewPeerConnection(cfg)
	require.NoError(t, err)
	pcAnswer, err := NewPeerConnection(cfg)
	require.NoError(t, err)

	defer closePairNow(t, pcOffer, pcAnswer)

	negotiationNeeded := make(chan struct{}, 1)
	pcOffer.OnNegotiationNeeded(func() {
		select {
		case negotiationNeeded <- struct{}{}:
		default:
		}
	})

	remoteDataChannel := make(chan *DataChannel, 1)
	pcAnswer.OnDataChannel(func(dc *DataChannel) {
		select {
		case remoteDataChannel <- dc:
		default:
		}
	})

	connectedWG := untilConnectionState(PeerConnectionStateConnected, pcOffer, pcAnswer)
	require.NoError(t, signalPairWithOptions(pcOffer, pcAnswer, withDisableInitialDataChannel(true)))

	connected := make(chan struct{})
	go func() {
		connectedWG.Wait()
		close(connected)
	}()

	select {
	case <-connected:
	case <-time.After(10 * time.Second):
		assert.FailNow(t, "connection establishment timed out")
	}

	// Verify no data channels initially exist
	pcOffer.sctpTransport.lock.Lock()
	offerDCCount := len(pcOffer.sctpTransport.dataChannels)
	pcOffer.sctpTransport.lock.Unlock()
	pcAnswer.sctpTransport.lock.Lock()
	answerDCCount := len(pcAnswer.sctpTransport.dataChannels)
	pcAnswer.sctpTransport.lock.Unlock()
	require.Equal(t, 0, offerDCCount)
	require.Equal(t, 0, answerDCCount)

	select {
	case <-remoteDataChannel:
		assert.FailNow(t, "unexpected OnDataChannel before CreateDataChannel")
	case <-time.After(100 * time.Millisecond):
	}

	// Now create a data channel and verify it works as expected
	localDC, err := pcOffer.CreateDataChannel("post-connect", nil)
	require.NoError(t, err)

	localOpened := make(chan struct{}, 1)
	localDC.OnOpen(func() {
		select {
		case localOpened <- struct{}{}:
		default:
		}
	})

	select {
	case <-negotiationNeeded:
		assert.FailNow(t, "unexpected OnNegotiationNeeded for CreateDataChannel")
	case <-time.After(250 * time.Millisecond):
	}

	var remoteDC *DataChannel
	select {
	case remoteDC = <-remoteDataChannel:
	case <-time.After(5 * time.Second):
		assert.FailNow(t, "timed out waiting for remote OnDataChannel")
	}

	remoteOpened := make(chan struct{}, 1)
	remoteDC.OnOpen(func() {
		select {
		case remoteOpened <- struct{}{}:
		default:
		}
	})

	select {
	case <-localOpened:
	case <-time.After(5 * time.Second):
		assert.FailNow(t, "timed out waiting for local data channel open")
	}

	select {
	case <-remoteOpened:
	case <-time.After(5 * time.Second):
		assert.FailNow(t, "timed out waiting for remote data channel open")
	}
}

func TestNoDuplicatedAttributesInMediaDescriptions(t *testing.T) { //nolint:cyclop
	pcOffer, err := NewPeerConnection(Configuration{})
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, pcOffer.Close())
	}()

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo)
	require.NoError(t, err)
	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeAudio)
	require.NoError(t, err)
	_, err = pcOffer.CreateDataChannel("initial_data_channel", nil)
	require.NoError(t, err)

	offer, err := pcOffer.CreateOffer(nil)
	require.NoError(t, err)

	for _, md := range offer.parsed.MediaDescriptions {
		attrs := make([]string, 0)
		for _, attr := range md.Attributes {
			str := fmt.Sprintf("%s %s", attr.Key, attr.Value)
			assert.Falsef(t, slices.Contains(attrs, str), "duplicate attribute found: %s", str)
			attrs = append(attrs, str)
		}
	}
}

func TestCryptexNegotiatedInSDP(t *testing.T) {
	var desc sdp.SessionDescription

	// Empty SDP
	desc = sdp.SessionDescription{}
	negotiatedForAnyMedia, negotiatedForAllMedia := cryptexNegotiatedInSDP(&desc)
	assert.False(t, negotiatedForAnyMedia)
	assert.False(t, negotiatedForAllMedia)

	// Cryptex in session-level attribute
	desc = sdp.SessionDescription{}
	desc.WithPropertyAttribute(sdp.AttrKeyCryptex)
	negotiatedForAnyMedia, negotiatedForAllMedia = cryptexNegotiatedInSDP(&desc)
	assert.True(t, negotiatedForAnyMedia)
	assert.True(t, negotiatedForAllMedia)

	// Cryptex in media-level attribute in all medias
	desc = sdp.SessionDescription{}
	media := &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media: "audio",
			Port:  sdp.RangedPort{Value: 9},
		},
	}
	media.WithPropertyAttribute(sdp.AttrKeyCryptex)
	desc.WithMedia(media)
	media2 := &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media: "video",
			Port:  sdp.RangedPort{Value: 9},
		},
	}
	media2.WithPropertyAttribute(sdp.AttrKeyCryptex)
	desc.WithMedia(media2)
	negotiatedForAnyMedia, negotiatedForAllMedia = cryptexNegotiatedInSDP(&desc)
	assert.True(t, negotiatedForAnyMedia)
	assert.True(t, negotiatedForAllMedia)

	// Cryptex in media-level attribute in some medias
	desc = sdp.SessionDescription{}
	media = &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media: "audio",
			Port:  sdp.RangedPort{Value: 9},
		},
	}
	media.WithPropertyAttribute(sdp.AttrKeyCryptex)
	desc.WithMedia(media)
	media2 = &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media: "video",
			Port:  sdp.RangedPort{Value: 9},
		},
	}
	desc.WithMedia(media2)
	negotiatedForAnyMedia, negotiatedForAllMedia = cryptexNegotiatedInSDP(&desc)
	assert.True(t, negotiatedForAnyMedia)
	assert.False(t, negotiatedForAllMedia)

	// Ignore application section
	desc = sdp.SessionDescription{}
	media = &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media: "audio",
			Port:  sdp.RangedPort{Value: 9},
		},
	}
	media.WithPropertyAttribute(sdp.AttrKeyCryptex)
	desc.WithMedia(media)
	media2 = &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media: "application",
			Port:  sdp.RangedPort{Value: 9},
		},
	}
	desc.WithMedia(media2)
	negotiatedForAnyMedia, negotiatedForAllMedia = cryptexNegotiatedInSDP(&desc)
	assert.True(t, negotiatedForAnyMedia)
	assert.True(t, negotiatedForAllMedia)

	// Ignore medias with port 0
	desc = sdp.SessionDescription{}
	media = &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media: "audio",
			Port:  sdp.RangedPort{Value: 9},
		},
	}
	media.WithPropertyAttribute(sdp.AttrKeyCryptex)
	desc.WithMedia(media)
	media2 = &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media: "video",
			Port:  sdp.RangedPort{Value: 0},
		},
	}
	desc.WithMedia(media2)
	negotiatedForAnyMedia, negotiatedForAllMedia = cryptexNegotiatedInSDP(&desc)
	assert.True(t, negotiatedForAnyMedia)
	assert.True(t, negotiatedForAllMedia)

	// No cryptex anywhere
	desc = sdp.SessionDescription{}
	media = &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media: "audio",
			Port:  sdp.RangedPort{Value: 9},
		},
	}
	desc.WithMedia(media)
	negotiatedForAnyMedia, negotiatedForAllMedia = cryptexNegotiatedInSDP(&desc)
	assert.False(t, negotiatedForAnyMedia)
	assert.False(t, negotiatedForAllMedia)
}

func TestCryptexAttributeInOffer(t *testing.T) {
	// Check if Cryptex attribute is correctly added to offer SDP based on RtpHeaderEncryptionPolicy
	// and SettingEngine.DisableRTPHeaderEncryption settings.

	tests := []RTPHeaderEncryptionPolicy{
		RTPHeaderEncryptionPolicyUnknown,
		RTPHeaderEncryptionPolicyDisable,
		RTPHeaderEncryptionPolicyNegotiate,
		RTPHeaderEncryptionPolicyRequire,
	}
	for _, policy := range tests {
		t.Run(policy.String(), func(t *testing.T) {
			pcOffer, err := NewPeerConnection(Configuration{
				RTPHeaderEncryptionPolicy: policy,
			})
			require.NoError(t, err)
			defer func() { require.NoError(t, pcOffer.Close()) }()

			_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo)
			require.NoError(t, err)
			_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeAudio)
			require.NoError(t, err)
			_, err = pcOffer.CreateDataChannel("initial_data_channel", nil)
			require.NoError(t, err)

			offer, err := pcOffer.CreateOffer(nil)
			require.NoError(t, err)

			assert.False(t, hasCryptexForSession(t, offer.SDP))

			hasVideoCryptex := hasCryptexForMedia(t, offer.SDP, string(MediaKindVideo))
			hasAudioCryptex := hasCryptexForMedia(t, offer.SDP, string(MediaKindAudio))
			hasApplicationCryptex := hasCryptexForMedia(t, offer.SDP, mediaSectionApplication)
			switch policy {
			case RTPHeaderEncryptionPolicyDisable:
				assert.False(t, hasVideoCryptex, "unexpected a=cryptex on offer video m-line")
				assert.False(t, hasAudioCryptex, "unexpected a=cryptex on offer audio m-line")
				assert.False(t, hasApplicationCryptex, "unexpected a=cryptex on offer application m-line")
			case RTPHeaderEncryptionPolicyUnknown, RTPHeaderEncryptionPolicyNegotiate, RTPHeaderEncryptionPolicyRequire:
				assert.True(t, hasVideoCryptex, "expected a=cryptex on offer video m-line")
				assert.True(t, hasAudioCryptex, "expected a=cryptex on offer audio m-line")
				assert.False(t, hasApplicationCryptex, "unexpected a=cryptex on offer application m-line")
			default:
				assert.FailNow(t, "Should not happen")
			}
		})
	}
}

//nolint:cyclop,dupl
func TestCryptexAttributeInAnswerForOfferWithCryptex(t *testing.T) {
	// Check if Cryptex attribute is correctly added to answer SDP based on RtpHeaderEncryptionPolicy
	// and SettingEngine.DisableRTPHeaderEncryption settings when offer contains Cryptex attribute.
	// Also check that answer is accepted.

	tests := []RTPHeaderEncryptionPolicy{
		RTPHeaderEncryptionPolicyUnknown,
		RTPHeaderEncryptionPolicyDisable,
		RTPHeaderEncryptionPolicyNegotiate,
		RTPHeaderEncryptionPolicyRequire,
	}
	for _, policy := range tests {
		t.Run(policy.String(), func(t *testing.T) {
			pcOffer, err := NewPeerConnection(Configuration{
				RTPHeaderEncryptionPolicy: RTPHeaderEncryptionPolicyNegotiate,
			})
			require.NoError(t, err)

			pcAnswer, err := NewPeerConnection(Configuration{
				RTPHeaderEncryptionPolicy: policy,
			})
			require.NoError(t, err)
			defer closePairNow(t, pcOffer, pcAnswer)

			offerVideoTr, err := pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo)
			require.NoError(t, err)
			offerAudioTr, err := pcOffer.AddTransceiverFromKind(RTPCodecTypeAudio)
			require.NoError(t, err)
			_, err = pcOffer.CreateDataChannel("initial_data_channel", nil)
			require.NoError(t, err)

			offer, err := pcOffer.CreateOffer(nil)
			require.NoError(t, err)
			offerGatheringComplete := GatheringCompletePromise(pcOffer)
			require.NoError(t, pcOffer.SetLocalDescription(offer))
			<-offerGatheringComplete
			offer = *pcOffer.LocalDescription()

			require.NoError(t, pcAnswer.SetRemoteDescription(offer))

			answer, err := pcAnswer.CreateAnswer(nil)
			require.NoError(t, err)
			answerGatheringComplete := GatheringCompletePromise(pcAnswer)
			require.NoError(t, pcAnswer.SetLocalDescription(answer))
			<-answerGatheringComplete
			answer = *pcAnswer.LocalDescription()

			assert.False(t, hasCryptexForSession(t, answer.SDP))

			hasVideoCryptex := hasCryptexForMedia(t, answer.SDP, string(MediaKindVideo))
			hasAudioCryptex := hasCryptexForMedia(t, answer.SDP, string(MediaKindAudio))
			hasApplicationCryptex := hasCryptexForMedia(t, answer.SDP, mediaSectionApplication)
			switch policy {
			case RTPHeaderEncryptionPolicyDisable:
				assert.False(t, hasVideoCryptex, "unexpected a=cryptex on offer video m-line")
				assert.False(t, hasAudioCryptex, "unexpected a=cryptex on offer audio m-line")
				assert.False(t, hasApplicationCryptex, "unexpected a=cryptex on offer application m-line")
			case RTPHeaderEncryptionPolicyUnknown, RTPHeaderEncryptionPolicyNegotiate, RTPHeaderEncryptionPolicyRequire:
				assert.True(t, hasVideoCryptex, "expected a=cryptex on offer video m-line")
				assert.True(t, hasAudioCryptex, "expected a=cryptex on offer audio m-line")
				assert.False(t, hasApplicationCryptex, "unexpected a=cryptex on offer application m-line")
			default:
				assert.FailNow(t, "Should not happen")
			}

			require.NoError(t, pcOffer.SetRemoteDescription(answer))

			waitForSRTPReady(t, pcAnswer)
			waitForSRTPReady(t, pcOffer)

			answerVideoTr := findTransceiverByKind(pcAnswer, RTPCodecTypeVideo)
			answerAudioTr := findTransceiverByKind(pcAnswer, RTPCodecTypeAudio)
			require.NotNil(t, answerVideoTr)
			require.NotNil(t, answerAudioTr)

			if policy == RTPHeaderEncryptionPolicyDisable {
				assert.Equal(t, srtp.CryptexModeDisabled, dtlsCryptexMode(pcOffer))
				assert.Equal(t, srtp.CryptexModeDisabled, dtlsCryptexMode(pcAnswer))

				assert.False(t, offerVideoTr.RTPHeaderEncryptionNegotiated())
				assert.False(t, offerAudioTr.RTPHeaderEncryptionNegotiated())
				assert.False(t, answerVideoTr.RTPHeaderEncryptionNegotiated())
				assert.False(t, answerAudioTr.RTPHeaderEncryptionNegotiated())
			} else {
				assert.Equal(t, srtp.CryptexModeEnabled, dtlsCryptexMode(pcOffer))
				if policy == RTPHeaderEncryptionPolicyRequire {
					assert.Equal(t, srtp.CryptexModeRequired, dtlsCryptexMode(pcAnswer))
				} else {
					assert.Equal(t, srtp.CryptexModeEnabled, dtlsCryptexMode(pcAnswer))
				}

				assert.True(t, offerVideoTr.RTPHeaderEncryptionNegotiated())
				assert.True(t, offerAudioTr.RTPHeaderEncryptionNegotiated())
				assert.True(t, answerVideoTr.RTPHeaderEncryptionNegotiated())
				assert.True(t, answerAudioTr.RTPHeaderEncryptionNegotiated())
			}
		})
	}
}

func TestCryptexAttributeInAnswerForOfferWithoutCryptex(t *testing.T) {
	// Check how answer SDP Cryptex attribute negotiation behaves based on RtpHeaderEncryptionPolicy
	// and SettingEngine.DisableRTPHeaderEncryption settings when offer does NOT contain Cryptex attribute.
	// Also check that answer is accepted.

	tests := []RTPHeaderEncryptionPolicy{
		RTPHeaderEncryptionPolicyUnknown,
		RTPHeaderEncryptionPolicyDisable,
		RTPHeaderEncryptionPolicyNegotiate,
		RTPHeaderEncryptionPolicyRequire,
	}
	for _, policy := range tests {
		t.Run(policy.String(), func(t *testing.T) {
			pcOffer, err := NewPeerConnection(Configuration{
				RTPHeaderEncryptionPolicy: RTPHeaderEncryptionPolicyDisable,
			})
			require.NoError(t, err)

			pcAnswer, err := NewPeerConnection(Configuration{
				RTPHeaderEncryptionPolicy: policy,
			})
			require.NoError(t, err)
			defer closePairNow(t, pcOffer, pcAnswer)

			offerVideoTr, err := pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo)
			require.NoError(t, err)
			offerAudioTr, err := pcOffer.AddTransceiverFromKind(RTPCodecTypeAudio)
			require.NoError(t, err)
			_, err = pcOffer.CreateDataChannel("initial_data_channel", nil)
			require.NoError(t, err)

			offer, err := pcOffer.CreateOffer(nil)
			require.NoError(t, err)
			offerGatheringComplete := GatheringCompletePromise(pcOffer)
			require.NoError(t, pcOffer.SetLocalDescription(offer))
			<-offerGatheringComplete
			offer = *pcOffer.LocalDescription()

			err = pcAnswer.SetRemoteDescription(offer)
			switch policy {
			case RTPHeaderEncryptionPolicyDisable, RTPHeaderEncryptionPolicyUnknown, RTPHeaderEncryptionPolicyNegotiate:
				var answer SessionDescription
				answer, err = pcAnswer.CreateAnswer(nil)
				require.NoError(t, err)
				answerGatheringComplete := GatheringCompletePromise(pcAnswer)
				require.NoError(t, pcAnswer.SetLocalDescription(answer))
				<-answerGatheringComplete
				answer = *pcAnswer.LocalDescription()

				require.NoError(t, err)
				hasVideoCryptex := hasCryptexForMedia(t, answer.SDP, string(MediaKindVideo))
				hasAudioCryptex := hasCryptexForMedia(t, answer.SDP, string(MediaKindAudio))
				hasApplicationCryptex := hasCryptexForMedia(t, answer.SDP, mediaSectionApplication)

				assert.False(t, hasVideoCryptex, "unexpected a=cryptex on offer video m-line")
				assert.False(t, hasAudioCryptex, "unexpected a=cryptex on offer audio m-line")
				assert.False(t, hasApplicationCryptex, "unexpected a=cryptex on offer application m-line")

				require.NoError(t, pcOffer.SetRemoteDescription(answer))

				waitForSRTPReady(t, pcAnswer)
				waitForSRTPReady(t, pcOffer)
				answerVideoTr := findTransceiverByKind(pcAnswer, RTPCodecTypeVideo)
				answerAudioTr := findTransceiverByKind(pcAnswer, RTPCodecTypeAudio)
				require.NotNil(t, answerVideoTr)
				require.NotNil(t, answerAudioTr)

				assert.Equal(t, srtp.CryptexModeDisabled, dtlsCryptexMode(pcOffer))
				assert.Equal(t, srtp.CryptexModeDisabled, dtlsCryptexMode(pcAnswer))

				assert.False(t, offerVideoTr.RTPHeaderEncryptionNegotiated())
				assert.False(t, offerAudioTr.RTPHeaderEncryptionNegotiated())
				assert.False(t, answerVideoTr.RTPHeaderEncryptionNegotiated())
				assert.False(t, answerAudioTr.RTPHeaderEncryptionNegotiated())
			case RTPHeaderEncryptionPolicyRequire:
				require.ErrorIs(t, err, ErrRTPHeaderEncryptionRequired, "expected error for offer without Cryptex")
			default:
				assert.FailNow(t, "Should not happen")
			}
		})
	}
}

func TestCryptexAttributeInAnswerMissingWhenOffererRequiresCryptex(t *testing.T) {
	// Check that answer SDP without Cryptex attribute is rejected when offerer requires Cryptex.

	pcOffer, err := NewPeerConnection(Configuration{
		RTPHeaderEncryptionPolicy: RTPHeaderEncryptionPolicyRequire,
	})
	require.NoError(t, err)

	pcAnswer, err := NewPeerConnection(Configuration{
		RTPHeaderEncryptionPolicy: RTPHeaderEncryptionPolicyDisable,
	})
	require.NoError(t, err)
	defer closePairNow(t, pcOffer, pcAnswer)

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo)
	require.NoError(t, err)
	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeAudio)
	require.NoError(t, err)
	_, err = pcOffer.CreateDataChannel("initial_data_channel", nil)
	require.NoError(t, err)

	offer, err := pcOffer.CreateOffer(nil)
	require.NoError(t, err)
	offerGatheringComplete := GatheringCompletePromise(pcOffer)
	require.NoError(t, pcOffer.SetLocalDescription(offer))
	<-offerGatheringComplete
	offer = *pcOffer.LocalDescription()

	require.NoError(t, pcAnswer.SetRemoteDescription(offer))

	answer, err := pcAnswer.CreateAnswer(nil)
	require.NoError(t, err)
	answerGatheringComplete := GatheringCompletePromise(pcAnswer)
	require.NoError(t, pcAnswer.SetLocalDescription(answer))
	<-answerGatheringComplete
	answer = *pcAnswer.LocalDescription()

	err = pcOffer.SetRemoteDescription(answer)
	require.ErrorIs(t, err, ErrRTPHeaderEncryptionRequired, "expected error for answer without Cryptex")
}

//nolint:cyclop,dupl
func TestCryptexAttributeInAnswerForSessionLevelCryptex(t *testing.T) {
	// Check if Cryptex attribute added at session-level in offer results in correct answer SDP
	// based on RtpHeaderEncryptionPolicy and SettingEngine.DisableRTPHeaderEncryption settings.
	// Also check that answer is accepted.

	tests := []RTPHeaderEncryptionPolicy{
		RTPHeaderEncryptionPolicyUnknown,
		RTPHeaderEncryptionPolicyDisable,
		RTPHeaderEncryptionPolicyNegotiate,
		RTPHeaderEncryptionPolicyRequire,
	}
	for _, policy := range tests {
		t.Run(policy.String(), func(t *testing.T) {
			pcOffer, err := NewPeerConnection(Configuration{
				RTPHeaderEncryptionPolicy: RTPHeaderEncryptionPolicyNegotiate,
			})
			require.NoError(t, err)

			pcAnswer, err := NewPeerConnection(Configuration{
				RTPHeaderEncryptionPolicy: policy,
			})
			require.NoError(t, err)
			defer closePairNow(t, pcOffer, pcAnswer)

			offerVideoTr, err := pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo)
			require.NoError(t, err)
			offerAudioTr, err := pcOffer.AddTransceiverFromKind(RTPCodecTypeAudio)
			require.NoError(t, err)
			_, err = pcOffer.CreateDataChannel("initial_data_channel", nil)
			require.NoError(t, err)

			offer, err := pcOffer.CreateOffer(nil)
			require.NoError(t, err)
			offerGatheringComplete := GatheringCompletePromise(pcOffer)
			require.NoError(t, pcOffer.SetLocalDescription(offer))
			<-offerGatheringComplete
			offer = *pcOffer.LocalDescription()

			// Manually add session-level cryptex attribute and remove media-level cryptex attributes from offer
			offerSDP := &sdp.SessionDescription{}
			require.NoError(t, offerSDP.Unmarshal([]byte(offer.SDP)))
			offerSDP.WithPropertyAttribute(sdp.AttrKeyCryptex)
			for _, md := range offerSDP.MediaDescriptions {
				md.Attributes = slices.DeleteFunc(md.Attributes, func(a sdp.Attribute) bool {
					return a.Key == sdp.AttrKeyCryptex
				})
			}
			offerSDPBytes, err := offerSDP.Marshal()
			require.NoError(t, err)
			offer.SDP = string(offerSDPBytes)

			require.NoError(t, pcAnswer.SetRemoteDescription(offer))

			answer, err := pcAnswer.CreateAnswer(nil)
			require.NoError(t, err)
			answerGatheringComplete := GatheringCompletePromise(pcAnswer)
			require.NoError(t, pcAnswer.SetLocalDescription(answer))
			<-answerGatheringComplete
			answer = *pcAnswer.LocalDescription()

			assert.False(t, hasCryptexForSession(t, answer.SDP))

			hasVideoCryptex := hasCryptexForMedia(t, answer.SDP, string(MediaKindVideo))
			hasAudioCryptex := hasCryptexForMedia(t, answer.SDP, string(MediaKindAudio))
			hasApplicationCryptex := hasCryptexForMedia(t, answer.SDP, mediaSectionApplication)
			switch policy {
			case RTPHeaderEncryptionPolicyDisable:
				assert.False(t, hasVideoCryptex, "unexpected a=cryptex on offer video m-line")
				assert.False(t, hasAudioCryptex, "unexpected a=cryptex on offer audio m-line")
				assert.False(t, hasApplicationCryptex, "unexpected a=cryptex on offer application m-line")
			case RTPHeaderEncryptionPolicyUnknown, RTPHeaderEncryptionPolicyNegotiate, RTPHeaderEncryptionPolicyRequire:
				assert.True(t, hasVideoCryptex, "expected a=cryptex on offer video m-line")
				assert.True(t, hasAudioCryptex, "expected a=cryptex on offer audio m-line")
				assert.False(t, hasApplicationCryptex, "unexpected a=cryptex on offer application m-line")
			default:
				assert.FailNow(t, "Should not happen")
			}

			require.NoError(t, pcOffer.SetRemoteDescription(answer))

			waitForSRTPReady(t, pcOffer)
			waitForSRTPReady(t, pcAnswer)
			answerVideoTr := findTransceiverByKind(pcAnswer, RTPCodecTypeVideo)
			answerAudioTr := findTransceiverByKind(pcAnswer, RTPCodecTypeAudio)
			require.NotNil(t, answerVideoTr)
			require.NotNil(t, answerAudioTr)

			if policy == RTPHeaderEncryptionPolicyDisable {
				assert.Equal(t, srtp.CryptexModeDisabled, dtlsCryptexMode(pcOffer))
				assert.Equal(t, srtp.CryptexModeDisabled, dtlsCryptexMode(pcAnswer))

				assert.False(t, offerVideoTr.RTPHeaderEncryptionNegotiated())
				assert.False(t, offerAudioTr.RTPHeaderEncryptionNegotiated())
				assert.False(t, answerVideoTr.RTPHeaderEncryptionNegotiated())
				assert.False(t, answerAudioTr.RTPHeaderEncryptionNegotiated())
			} else {
				assert.Equal(t, srtp.CryptexModeEnabled, dtlsCryptexMode(pcOffer))
				if policy == RTPHeaderEncryptionPolicyRequire {
					assert.Equal(t, srtp.CryptexModeRequired, dtlsCryptexMode(pcAnswer))
				} else {
					assert.Equal(t, srtp.CryptexModeEnabled, dtlsCryptexMode(pcAnswer))
				}

				assert.True(t, offerVideoTr.RTPHeaderEncryptionNegotiated())
				assert.True(t, offerAudioTr.RTPHeaderEncryptionNegotiated())
				assert.True(t, answerVideoTr.RTPHeaderEncryptionNegotiated())
				assert.True(t, answerAudioTr.RTPHeaderEncryptionNegotiated())
			}
		})
	}
}
