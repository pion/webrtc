// +build !js

package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/pions/ice"
	"github.com/pions/transport/test"
	"github.com/pions/webrtc/internal/mux"
	"github.com/pions/webrtc/pkg/rtcerr"
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
			}, &rtcerr.InvalidAccessError{Err: ErrNoTurnCredencials}},
		}

		for i, testCase := range testCases {
			_, err := testCase.initialize()
			assert.EqualError(t, err, testCase.expectedErr.Error(),
				"testCase: %d %v", i, testCase,
			)
		}
	})
}

func TestPeerConnection_SetConfiguration_Go(t *testing.T) {
	// Note: this test includes all SetConfiguration features that are supported
	// by Go but not the WASM bindings, namely: ICEServer.Credential,
	// ICEServer.CredentialType, and Certificates.

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
			wantErr: &rtcerr.InvalidAccessError{Err: ErrNoTurnCredencials},
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
	// Note: When testing the Go event handlers we peer into the state a bit more
	// than what is possible for the environment agnostic (Go or WASM/JavaScript)
	// EventHandlers test.
	api := NewAPI()
	pc, err := api.NewPeerConnection(Configuration{})
	assert.Nil(t, err)

	onTrackCalled := make(chan bool)
	onICEConnectionStateChangeCalled := make(chan bool)
	onDataChannelCalled := make(chan bool)

	// Verify that the noop case works
	assert.NotPanics(t, func() { pc.onTrack(nil, nil) })
	assert.NotPanics(t, func() { pc.onICEConnectionStateChange(ice.ConnectionStateNew) })

	pc.OnTrack(func(t *Track, r *RTPReceiver) {
		onTrackCalled <- true
	})

	pc.OnICEConnectionStateChange(func(cs ICEConnectionState) {
		onICEConnectionStateChangeCalled <- true
	})

	pc.OnDataChannel(func(dc *DataChannel) {
		onDataChannelCalled <- true
	})

	// Verify that the handlers deal with nil inputs
	assert.NotPanics(t, func() { pc.onTrack(nil, nil) })
	assert.NotPanics(t, func() { go pc.onDataChannelHandler(nil) })

	// Verify that the set handlers are called
	assert.NotPanics(t, func() { pc.onTrack(&Track{}, &RTPReceiver{}) })
	assert.NotPanics(t, func() { pc.onICEConnectionStateChange(ice.ConnectionStateNew) })
	assert.NotPanics(t, func() { go pc.onDataChannelHandler(&DataChannel{api: api}) })

	allTrue := func(vals []bool) bool {
		for _, val := range vals {
			if !val {
				return false
			}
		}
		return true
	}

	assert.True(t, allTrue([]bool{
		<-onTrackCalled,
		<-onICEConnectionStateChangeCalled,
		<-onDataChannelCalled,
	}))
}

// This test asserts that nothing deadlocks we try to shutdown when DTLS is in flight
// We ensure that DTLS is in flight by removing the mux func for it, so all inbound DTLS is lost
func TestPeerConnection_ShutdownNoDTLS(t *testing.T) {
	dtlsMatchFunc := mux.MatchDTLS
	defer func() {
		mux.MatchDTLS = dtlsMatchFunc
	}()

	// Drop all incoming DTLS traffic
	mux.MatchDTLS = func([]byte) bool {
		return false
	}

	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	api := NewAPI()
	offerPC, answerPC, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

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
	if err = offerPC.Close(); err != nil {
		t.Fatal(err)
	} else if err = answerPC.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPeerConnection_PeropertyGetters(t *testing.T) {
	pc := &PeerConnection{
		currentLocalDescription:  &SessionDescription{},
		pendingLocalDescription:  &SessionDescription{},
		currentRemoteDescription: &SessionDescription{},
		pendingRemoteDescription: &SessionDescription{},
		signalingState:           SignalingStateHaveLocalOffer,
		iceGatheringState:        ICEGatheringStateGathering,
		iceConnectionState:       ICEConnectionStateChecking,
		connectionState:          PeerConnectionStateConnecting,
	}

	assert.Equal(t, pc.currentLocalDescription, pc.CurrentLocalDescription(), "should match")
	assert.Equal(t, pc.pendingLocalDescription, pc.PendingLocalDescription(), "should match")
	assert.Equal(t, pc.currentRemoteDescription, pc.CurrentRemoteDescription(), "should match")
	assert.Equal(t, pc.pendingRemoteDescription, pc.PendingRemoteDescription(), "should match")
	assert.Equal(t, pc.signalingState, pc.SignalingState(), "should match")
	assert.Equal(t, pc.iceGatheringState, pc.ICEGatheringState(), "should match")
	assert.Equal(t, pc.iceConnectionState, pc.ICEConnectionState(), "should match")
	assert.Equal(t, pc.connectionState, pc.ConnectionState(), "should match")
}
