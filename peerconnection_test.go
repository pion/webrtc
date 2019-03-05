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

	"github.com/pions/webrtc/internal/ice"

	"github.com/pions/webrtc/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
)

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

func signalPair(pcOffer *PeerConnection, pcAnswer *PeerConnection) error {
	offer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		return err
	}

	if err = pcOffer.SetLocalDescription(offer); err != nil {
		return err
	}

	err = pcAnswer.SetRemoteDescription(offer)
	if err != nil {
		return err
	}

	answer, err := pcAnswer.CreateAnswer(nil)
	if err != nil {
		return err
	}

	if err = pcAnswer.SetLocalDescription(answer); err != nil {
		return err
	}

	err = pcOffer.SetRemoteDescription(answer)
	if err != nil {
		return err
	}

	return nil
}

func TestNew(t *testing.T) {
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

func TestPeerConnection_SetConfiguration(t *testing.T) {
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
			name: "closed connection",
			init: func() (*PeerConnection, error) {
				pc, err := api.NewPeerConnection(Configuration{})
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
				return api.NewPeerConnection(Configuration{})
			},
			config: Configuration{
				PeerIdentity: "unittest",
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingPeerIdentity},
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
			name: "update BundlePolicy",
			init: func() (*PeerConnection, error) {
				return api.NewPeerConnection(Configuration{})
			},
			config: Configuration{
				BundlePolicy: BundlePolicyMaxCompat,
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingBundlePolicy},
		},
		{
			name: "update RTCPMuxPolicy",
			init: func() (*PeerConnection, error) {
				return api.NewPeerConnection(Configuration{})
			},
			config: Configuration{
				RTCPMuxPolicy: RTCPMuxPolicyNegotiate,
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingRTCPMuxPolicy},
		},
		{
			name: "update ICECandidatePoolSize",
			init: func() (*PeerConnection, error) {
				pc, err := api.NewPeerConnection(Configuration{
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
			name: "update ICEServers, no TURN credentials",
			init: func() (*PeerConnection, error) {
				return api.NewPeerConnection(Configuration{})
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
			t.Fatalf("SetConfiguration %q: init failed: %v", test.name, err)
		}

		err = pc.SetConfiguration(test.config)
		if got, want := err, test.wantErr; !reflect.DeepEqual(got, want) {
			t.Fatalf("SetConfiguration %q: err = %v, want %v", test.name, got, want)
		}
	}
}

func TestPeerConnection_GetConfiguration(t *testing.T) {
	api := NewAPI()
	pc, err := api.NewPeerConnection(Configuration{})
	assert.Nil(t, err)

	expected := Configuration{
		ICEServers:           []ICEServer{},
		ICETransportPolicy:   ICETransportPolicyAll,
		BundlePolicy:         BundlePolicyBalanced,
		RTCPMuxPolicy:        RTCPMuxPolicyRequire,
		Certificates:         []Certificate{},
		ICECandidatePoolSize: 0,
	}
	actual := pc.GetConfiguration()
	assert.True(t, &expected != &actual)
	assert.Equal(t, expected.ICEServers, actual.ICEServers)
	assert.Equal(t, expected.ICETransportPolicy, actual.ICETransportPolicy)
	assert.Equal(t, expected.BundlePolicy, actual.BundlePolicy)
	assert.Equal(t, expected.RTCPMuxPolicy, actual.RTCPMuxPolicy)
	assert.NotEqual(t, len(expected.Certificates), len(actual.Certificates))
	assert.Equal(t, expected.ICECandidatePoolSize, actual.ICECandidatePoolSize)
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

// TODO Fix this test
const minimalOffer = `v=0
o=- 7193157174393298413 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE video
m=video 43858 UDP/TLS/RTP/SAVPF 96
c=IN IP4 172.17.0.1
a=candidate:3885250869 1 udp 1 127.0.0.1 1 typ host
a=ice-ufrag:OgYk
a=ice-pwd:G0ka4ts7hRhMLNljuuXzqnOF
a=fingerprint:sha-256 D7:06:10:DE:69:66:B1:53:0E:02:33:45:63:F8:AF:78:B2:C7:CE:AF:8E:FD:E5:13:20:50:74:93:CD:B5:C8:69
a=setup:active
a=mid:video
a=sendrecv
a=rtpmap:96 VP8/90000
`

func TestSetRemoteDescription(t *testing.T) {
	api := NewAPI()
	testCases := []struct {
		desc SessionDescription
	}{
		{SessionDescription{Type: SDPTypeOffer, SDP: minimalOffer}},
	}

	for i, testCase := range testCases {
		peerConn, err := api.NewPeerConnection(Configuration{})
		if err != nil {
			t.Errorf("Case %d: got error: %v", i, err)
		}
		err = peerConn.SetRemoteDescription(testCase.desc)
		if err != nil {
			t.Errorf("Case %d: got error: %v", i, err)
		}
	}
}

func TestCreateOfferAnswer(t *testing.T) {
	api := NewAPI()
	offerPeerConn, err := api.NewPeerConnection(Configuration{})
	if err != nil {
		t.Errorf("New PeerConnection: got error: %v", err)
	}
	offer, err := offerPeerConn.CreateOffer(nil)
	if err != nil {
		t.Errorf("Create Offer: got error: %v", err)
	}
	if err = offerPeerConn.SetLocalDescription(offer); err != nil {
		t.Errorf("SetLocalDescription: got error: %v", err)
	}
	answerPeerConn, err := api.NewPeerConnection(Configuration{})
	if err != nil {
		t.Errorf("New PeerConnection: got error: %v", err)
	}
	err = answerPeerConn.SetRemoteDescription(offer)
	if err != nil {
		t.Errorf("SetRemoteDescription: got error: %v", err)
	}
	answer, err := answerPeerConn.CreateAnswer(nil)
	if err != nil {
		t.Errorf("Create Answer: got error: %v", err)
	}
	if err = answerPeerConn.SetLocalDescription(answer); err != nil {
		t.Errorf("SetLocalDescription: got error: %v", err)
	}
	err = offerPeerConn.SetRemoteDescription(answer)
	if err != nil {
		t.Errorf("SetRemoteDescription (Originator): got error: %v", err)
	}
}

func TestPeerConnection_EventHandlers(t *testing.T) {
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
