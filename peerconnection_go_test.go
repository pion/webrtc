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

// TODO - This unittest needs to be completed when CreateDataChannel is complete
// func TestPeerConnection_CreateDataChannel(t *testing.T) {
// 	pc, err := New(Configuration{})
// 	assert.Nil(t, err)
//
// 	_, err = pc.CreateDataChannel("data", &DataChannelInit{
//
// 	})
// 	assert.Nil(t, err)
// }

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
		return &RTPTransceiver{kind: kind, Direction: direction}
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

	_, err = pc.AddTransceiver(RTPCodecTypeVideo)
	assert.NoError(t, err)

	_, err = pc.AddTransceiver(RTPCodecTypeAudio)
	assert.NoError(t, err)

	_, err = pc.AddTransceiver(RTPCodecTypeAudio)
	assert.NoError(t, err)

	_, err = pc.AddTransceiver(RTPCodecTypeVideo)
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

// An invalid fingerprint MUST cause PeerConnectionState to go to PeerConnectionStateFailed
func TestInvalidFingerprintCausesFailed(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	pcOffer, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	pcAnswer, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	offerChan := make(chan SessionDescription)
	pcOffer.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			offerChan <- *pcOffer.PendingLocalDescription()
		}
	})

	connectionHasFailed, closeFunc := context.WithCancel(context.Background())
	pcAnswer.OnConnectionStateChange(func(connectionState PeerConnectionState) {
		if connectionState == PeerConnectionStateFailed {
			closeFunc()
		}
	})

	if _, err = pcOffer.CreateDataChannel("unusedDataChannel", nil); err != nil {
		t.Fatal(err)
	}

	offer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	} else if err := pcOffer.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}

	select {
	case offer := <-offerChan:
		// Replace with invalid fingerprint
		re := regexp.MustCompile(`sha-256 (.*?)\r`)
		offer.SDP = re.ReplaceAllString(offer.SDP, "sha-256 AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA\r")

		if err := pcAnswer.SetRemoteDescription(offer); err != nil {
			t.Fatal(err)
		}

		answer, err := pcAnswer.CreateAnswer(nil)
		if err != nil {
			t.Fatal(err)
		}

		if err = pcAnswer.SetLocalDescription(answer); err != nil {
			t.Fatal(err)
		}

		err = pcOffer.SetRemoteDescription(answer)
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting to receive offer")
	}

	select {
	case <-connectionHasFailed.Done():
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for connection to fail")
	}

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}

func TestPeerConnection_DTLSRoleSettingEngine(t *testing.T) {
	runTest := func(r DTLSRole) {
		s := SettingEngine{}
		assert.NoError(t, s.SetAnsweringDTLSRole(r))

		offerPC, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
		if err != nil {
			t.Fatal(err)
		}

		answerPC, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
		if err != nil {
			t.Fatal(err)
		}

		if err = signalPair(offerPC, answerPC); err != nil {
			t.Fatal(err)
		}

		connectionComplete := make(chan interface{})
		answerPC.OnConnectionStateChange(func(connectionState PeerConnectionState) {
			if connectionState == PeerConnectionStateConnected {
				select {
				case <-connectionComplete:
				default:
					close(connectionComplete)
				}
			}
		})

		<-connectionComplete
		assert.NoError(t, offerPC.Close())
		assert.NoError(t, answerPC.Close())
	}

	report := test.CheckRoutines(t)
	defer report()

	t.Run("Server", func(t *testing.T) {
		runTest(DTLSRoleServer)
	})

	t.Run("Client", func(t *testing.T) {
		runTest(DTLSRoleClient)
	})

}
