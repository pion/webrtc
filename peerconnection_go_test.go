// +build !js

package webrtc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/pion/ice"
	"github.com/pion/transport/test"
	"github.com/pion/webrtc/v2/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
)

// newPair creates two new peer connections (an offerer and an answerer) using
// the api.
func (api *API) newPair() (pcOffer *PeerConnection, pcAnswer *PeerConnection, err error) {
	pca, err := api.NewPeerConnection(Configuration{})
	if err != nil {
		return nil, nil, err
	}

	pcb, err := api.NewPeerConnection(Configuration{})
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
	assert.NotPanics(t, func() { pc.onICEConnectionStateChange(ice.ConnectionStateNew) })

	pc.OnTrack(func(t *Track, r *RTPReceiver) {
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
	assert.NotPanics(t, func() { pc.onTrack(&Track{}, &RTPReceiver{}) })
	assert.NotPanics(t, func() { pc.onICEConnectionStateChange(ice.ConnectionStateNew) })
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
	offerPC, answerPC, err := api.newPair()
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
	assert.NoError(t, offerPC.Close())
	assert.NoError(t, answerPC.Close())
}

func TestPeerConnection_PropertyGetters(t *testing.T) {
	pc := &PeerConnection{
		currentLocalDescription:  &SessionDescription{},
		pendingLocalDescription:  &SessionDescription{},
		currentRemoteDescription: &SessionDescription{},
		pendingRemoteDescription: &SessionDescription{},
		signalingState:           SignalingStateHaveLocalOffer,
		iceConnectionState:       ICEConnectionStateChecking,
		connectionState:          PeerConnectionStateConnecting,
	}

	assert.Equal(t, pc.currentLocalDescription, pc.CurrentLocalDescription(), "should match")
	assert.Equal(t, pc.pendingLocalDescription, pc.PendingLocalDescription(), "should match")
	assert.Equal(t, pc.currentRemoteDescription, pc.CurrentRemoteDescription(), "should match")
	assert.Equal(t, pc.pendingRemoteDescription, pc.PendingRemoteDescription(), "should match")
	assert.Equal(t, pc.signalingState, pc.SignalingState(), "should match")
	assert.Equal(t, pc.iceConnectionState, pc.ICEConnectionState(), "should match")
	assert.Equal(t, pc.connectionState, pc.ConnectionState(), "should match")
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
			[]*RTPTransceiver{createTransceiver(RTPCodecTypeVideo, RTPTransceiverDirectionInactive)},
		},
		{
			"No local Transceivers, every remote should get an inactive",
			[]RTPCodecType{RTPCodecTypeVideo, RTPCodecTypeAudio, RTPCodecTypeVideo, RTPCodecTypeVideo},
			[]RTPTransceiverDirection{RTPTransceiverDirectionSendrecv, RTPTransceiverDirectionRecvonly, RTPTransceiverDirectionSendonly, RTPTransceiverDirectionInactive},

			[]*RTPTransceiver{},

			[]*RTPTransceiver{
				createTransceiver(RTPCodecTypeVideo, RTPTransceiverDirectionInactive),
				createTransceiver(RTPCodecTypeAudio, RTPTransceiverDirectionInactive),
				createTransceiver(RTPCodecTypeVideo, RTPTransceiverDirectionInactive),
				createTransceiver(RTPCodecTypeVideo, RTPTransceiverDirectionInactive),
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

	// 5 because a datachannel is always added
	assert.Len(t, matches, 5)
	assert.NoError(t, pc.Close())
}

// Assert that candidates are gathered by calling SetLocalDescription, not SetRemoteDescription
// When trickle in on by default we can move this to peerconnection_test.go
func TestGatherOnSetLocalDescription(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOfferGathered := make(chan SessionDescription)
	pcAnswerGathered := make(chan SessionDescription)

	s := SettingEngine{}
	s.SetTrickle(true)
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
	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}

func TestPeerConnection_OfferingLite(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	s := SettingEngine{}
	s.SetLite(true)
	offerPC, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	answerPC, err := NewAPI().NewPeerConnection(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	if err = signalPair(offerPC, answerPC); err != nil {
		t.Fatal(err)
	}

	iceComplete := make(chan interface{})
	answerPC.OnICEConnectionStateChange(func(iceState ICEConnectionState) {
		if iceState == ICEConnectionStateConnected {
			select {
			case <-iceComplete:
			default:
				close(iceComplete)
			}
		}
	})

	<-iceComplete
	assert.NoError(t, offerPC.Close())
	assert.NoError(t, answerPC.Close())
}

func TestPeerConnection_AnsweringLite(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	offerPC, err := NewAPI().NewPeerConnection(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	s := SettingEngine{}
	s.SetLite(true)
	answerPC, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	if err = signalPair(offerPC, answerPC); err != nil {
		t.Fatal(err)
	}

	iceComplete := make(chan interface{})
	answerPC.OnICEConnectionStateChange(func(iceState ICEConnectionState) {
		if iceState == ICEConnectionStateConnected {
			select {
			case <-iceComplete:
			default:
				close(iceComplete)
			}
		}
	})

	<-iceComplete
	assert.NoError(t, offerPC.Close())
	assert.NoError(t, answerPC.Close())
}

func TestOnICEGatheringStateChange(t *testing.T) {
	seenGathering := &atomicBool{}
	seenComplete := &atomicBool{}

	seenGatheringAndComplete := make(chan interface{})
	seenClosed := make(chan interface{})

	s := SettingEngine{}
	s.SetTrickle(true)

	peerConn, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
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

		switch s {
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

// Assert that when Trickle is enabled two connections can connect
func TestPeerConnectionTrickle(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	s := SettingEngine{}
	s.SetTrickle(true)

	api := NewAPI(WithSettingEngine(s))
	offerPC, answerPC, err := api.newPair()
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
	assert.NoError(t, offerPC.Close())
	assert.NoError(t, answerPC.Close())
}

// Issue #1121, assert populateLocalCandidates doesn't mutate
func TestPopulateLocalCandidates(t *testing.T) {
	t.Run("PendingLocalDescription shouldn't add extra mutations", func(t *testing.T) {
		pc, err := NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		offer, err := pc.CreateOffer(nil)
		assert.NoError(t, err)

		assert.NoError(t, pc.SetLocalDescription(offer))
		assert.Equal(t, pc.PendingLocalDescription(), pc.PendingLocalDescription())
		assert.NoError(t, pc.Close())
	})

	t.Run("end-of-candidates only when gathering is complete", func(t *testing.T) {
		s := SettingEngine{}
		s.SetTrickle(true)

		pc, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		gatherComplete, gatherCompleteCancel := context.WithCancel(context.Background())
		pc.OnICEGatheringStateChange(func(i ICEGathererState) {
			if i == ICEGathererStateComplete {
				gatherCompleteCancel()
			}
		})

		offer, err := pc.CreateOffer(nil)
		assert.NoError(t, err)
		assert.NotContains(t, offer.SDP, "a=candidate")
		assert.NotContains(t, offer.SDP, "a=end-of-candidates")

		assert.NoError(t, pc.SetLocalDescription(offer))
		<-gatherComplete.Done()
		assert.Contains(t, pc.PendingLocalDescription().SDP, "a=candidate")
		assert.Contains(t, pc.PendingLocalDescription().SDP, "a=end-of-candidates")

		assert.NoError(t, pc.Close())
	})
}
