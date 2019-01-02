package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"math/big"
	"testing"
	"time"

	"github.com/pions/webrtc/pkg/ice"

	"github.com/pions/webrtc/pkg/media"
	"github.com/pions/webrtc/pkg/rtp"

	"github.com/pions/webrtc/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
)

func newPair() (pcOffer *RTCPeerConnection, pcAnswer *RTCPeerConnection, err error) {
	pca, err := New(RTCConfiguration{})
	if err != nil {
		return nil, nil, err
	}

	pcb, err := New(RTCConfiguration{})
	if err != nil {
		return nil, nil, err
	}

	return pca, pcb, nil
}

func signalPair(pcOffer *RTCPeerConnection, pcAnswer *RTCPeerConnection) error {
	offer, err := pcOffer.CreateOffer(nil)
	if err != nil {
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

		pc, err := api.New(RTCConfiguration{
			IceServers: []RTCIceServer{
				{
					URLs: []string{
						"stun:stun.l.google.com:19302",
						"turns:google.de?transport=tcp",
					},
					Username: "unittest",
					Credential: RTCOAuthCredential{
						MacKey:      "WmtzanB3ZW9peFhtdm42NzUzNG0=",
						AccessToken: "AAwg3kPHWPfvk9bDFL936wYvkoctMADzQ==",
					},
					CredentialType: RTCIceCredentialTypeOauth,
				},
			},
			IceTransportPolicy:   RTCIceTransportPolicyRelay,
			BundlePolicy:         RTCBundlePolicyMaxCompat,
			RtcpMuxPolicy:        RTCRtcpMuxPolicyNegotiate,
			PeerIdentity:         "unittest",
			Certificates:         []RTCCertificate{*certificate},
			IceCandidatePoolSize: 5,
		})
		assert.Nil(t, err)
		assert.NotNil(t, pc)
	})
	t.Run("Failure", func(t *testing.T) {
		testCases := []struct {
			initialize  func() (*RTCPeerConnection, error)
			expectedErr error
		}{
			{func() (*RTCPeerConnection, error) {
				secretKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				assert.Nil(t, err)

				certificate, err := NewRTCCertificate(secretKey, x509.Certificate{
					Version:      2,
					SerialNumber: big.NewInt(1653),
					NotBefore:    time.Now().AddDate(0, -2, 0),
					NotAfter:     time.Now().AddDate(0, -1, 0),
				})
				assert.Nil(t, err)

				return api.New(RTCConfiguration{
					Certificates: []RTCCertificate{*certificate},
				})
			}, &rtcerr.InvalidAccessError{Err: ErrCertificateExpired}},
			{func() (*RTCPeerConnection, error) {
				return api.New(RTCConfiguration{
					IceServers: []RTCIceServer{
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

func TestRTCPeerConnection_SetConfiguration(t *testing.T) {
	api := NewAPI()
	t.Run("Success", func(t *testing.T) {
		secretKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		assert.Nil(t, err)

		certificate, err := GenerateCertificate(secretKey)
		assert.Nil(t, err)

		pc, err := api.New(RTCConfiguration{
			PeerIdentity:         "unittest",
			Certificates:         []RTCCertificate{*certificate},
			IceCandidatePoolSize: 5,
		})
		assert.Nil(t, err)

		err = pc.SetConfiguration(RTCConfiguration{
			IceServers: []RTCIceServer{
				{
					URLs: []string{
						"stun:stun.l.google.com:19302",
						"turns:google.de?transport=tcp",
					},
					Username: "unittest",
					Credential: RTCOAuthCredential{
						MacKey:      "WmtzanB3ZW9peFhtdm42NzUzNG0=",
						AccessToken: "AAwg3kPHWPfvk9bDFL936wYvkoctMADzQ==",
					},
					CredentialType: RTCIceCredentialTypeOauth,
				},
			},
			IceTransportPolicy:   RTCIceTransportPolicyAll,
			BundlePolicy:         RTCBundlePolicyBalanced,
			RtcpMuxPolicy:        RTCRtcpMuxPolicyRequire,
			PeerIdentity:         "unittest",
			Certificates:         []RTCCertificate{*certificate},
			IceCandidatePoolSize: 5,
		})
		assert.Nil(t, err)
	})
	t.Run("Failure", func(t *testing.T) {
		testCases := []struct {
			initialize     func() (*RTCPeerConnection, error)
			updatingConfig func() RTCConfiguration
			expectedErr    error
		}{
			{func() (*RTCPeerConnection, error) {
				pc, err := api.New(RTCConfiguration{})
				assert.Nil(t, err)

				err = pc.Close()
				assert.Nil(t, err)
				return pc, err
			}, func() RTCConfiguration {
				return RTCConfiguration{}
			}, &rtcerr.InvalidStateError{Err: ErrConnectionClosed}},
			{func() (*RTCPeerConnection, error) {
				return api.New(RTCConfiguration{})
			}, func() RTCConfiguration {
				return RTCConfiguration{
					PeerIdentity: "unittest",
				}
			}, &rtcerr.InvalidModificationError{Err: ErrModifyingPeerIdentity}},
			{func() (*RTCPeerConnection, error) {
				return New(RTCConfiguration{})
			}, func() RTCConfiguration {
				secretKey1, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				assert.Nil(t, err)

				certificate1, err := GenerateCertificate(secretKey1)
				assert.Nil(t, err)

				secretKey2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				assert.Nil(t, err)

				certificate2, err := GenerateCertificate(secretKey2)
				assert.Nil(t, err)

				return RTCConfiguration{
					Certificates: []RTCCertificate{*certificate1, *certificate2},
				}
			}, &rtcerr.InvalidModificationError{Err: ErrModifyingCertificates}},
			{func() (*RTCPeerConnection, error) {
				return New(RTCConfiguration{})
			}, func() RTCConfiguration {
				secretKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				assert.Nil(t, err)

				certificate, err := GenerateCertificate(secretKey)
				assert.Nil(t, err)

				return RTCConfiguration{
					Certificates: []RTCCertificate{*certificate},
				}
			}, &rtcerr.InvalidModificationError{Err: ErrModifyingCertificates}},
			{func() (*RTCPeerConnection, error) {
				return New(RTCConfiguration{})
			}, func() RTCConfiguration {
				return RTCConfiguration{
					BundlePolicy: RTCBundlePolicyMaxCompat,
				}
			}, &rtcerr.InvalidModificationError{Err: ErrModifyingBundlePolicy}},
			{func() (*RTCPeerConnection, error) {
				return New(RTCConfiguration{})
			}, func() RTCConfiguration {
				return RTCConfiguration{
					RtcpMuxPolicy: RTCRtcpMuxPolicyNegotiate,
				}
			}, &rtcerr.InvalidModificationError{Err: ErrModifyingRtcpMuxPolicy}},
			// TODO Unittest for IceCandidatePoolSize cannot be done now needs pc.LocalDescription()
			{func() (*RTCPeerConnection, error) {
				return New(RTCConfiguration{})
			}, func() RTCConfiguration {
				return RTCConfiguration{
					IceServers: []RTCIceServer{
						{
							URLs: []string{
								"stun:stun.l.google.com:19302",
								"turns:google.de?transport=tcp",
							},
							Username: "unittest",
						},
					},
				}
			}, &rtcerr.InvalidAccessError{Err: ErrNoTurnCredencials}},
		}

		for i, testCase := range testCases {
			pc, err := testCase.initialize()
			assert.Nil(t, err)

			err = pc.SetConfiguration(testCase.updatingConfig())
			assert.EqualError(t, err, testCase.expectedErr.Error(),
				"testCase: %d %v", i, testCase,
			)
		}
	})
}

func TestRTCPeerConnection_GetConfiguration(t *testing.T) {
	pc, err := New(RTCConfiguration{})
	assert.Nil(t, err)

	expected := RTCConfiguration{
		IceServers:           []RTCIceServer{},
		IceTransportPolicy:   RTCIceTransportPolicyAll,
		BundlePolicy:         RTCBundlePolicyBalanced,
		RtcpMuxPolicy:        RTCRtcpMuxPolicyRequire,
		Certificates:         []RTCCertificate{},
		IceCandidatePoolSize: 0,
	}
	actual := pc.GetConfiguration()
	assert.True(t, &expected != &actual)
	assert.Equal(t, expected.IceServers, actual.IceServers)
	assert.Equal(t, expected.IceTransportPolicy, actual.IceTransportPolicy)
	assert.Equal(t, expected.BundlePolicy, actual.BundlePolicy)
	assert.Equal(t, expected.RtcpMuxPolicy, actual.RtcpMuxPolicy)
	assert.NotEqual(t, len(expected.Certificates), len(actual.Certificates))
	assert.Equal(t, expected.IceCandidatePoolSize, actual.IceCandidatePoolSize)
}

// TODO - This unittest needs to be completed when CreateDataChannel is complete
// func TestRTCPeerConnection_CreateDataChannel(t *testing.T) {
// 	pc, err := New(RTCConfiguration{})
// 	assert.Nil(t, err)
//
// 	_, err = pc.CreateDataChannel("data", &RTCDataChannelInit{
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
	testCases := []struct {
		desc RTCSessionDescription
	}{
		{RTCSessionDescription{Type: RTCSdpTypeOffer, Sdp: minimalOffer}},
	}

	for i, testCase := range testCases {
		peerConn, err := New(RTCConfiguration{})
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
	offerPeerConn, err := New(RTCConfiguration{})
	if err != nil {
		t.Errorf("New RTCPeerConnection: got error: %v", err)
	}
	offer, err := offerPeerConn.CreateOffer(nil)
	if err != nil {
		t.Errorf("Create Offer: got error: %v", err)
	}
	answerPeerConn, err := New(RTCConfiguration{})
	if err != nil {
		t.Errorf("New RTCPeerConnection: got error: %v", err)
	}
	err = answerPeerConn.SetRemoteDescription(offer)
	if err != nil {
		t.Errorf("SetRemoteDescription: got error: %v", err)
	}
	answer, err := answerPeerConn.CreateAnswer(nil)
	if err != nil {
		t.Errorf("Create Answer: got error: %v", err)
	}
	err = offerPeerConn.SetRemoteDescription(answer)
	if err != nil {
		t.Errorf("SetRemoteDescription (Originator): got error: %v", err)
	}
}

func TestRTCPeerConnection_NewRawRTPTrack(t *testing.T) {
	RegisterDefaultCodecs()

	pc, err := New(RTCConfiguration{})
	assert.Nil(t, err)

	_, err = pc.NewRawRTPTrack(DefaultPayloadTypeH264, 0, "trackId", "trackLabel")
	assert.NotNil(t, err)

	track, err := pc.NewRawRTPTrack(DefaultPayloadTypeH264, 123456, "trackId", "trackLabel")
	assert.Nil(t, err)

	// This channel should not be set up for a RawRTP track
	assert.Panics(t, func() {
		track.Samples <- media.RTCSample{}
	})

	assert.NotPanics(t, func() {
		track.RawRTP <- &rtp.Packet{}
	})
}

func TestRTCPeerConnection_NewRTCSampleTrack(t *testing.T) {
	RegisterDefaultCodecs()

	pc, err := New(RTCConfiguration{})
	assert.Nil(t, err)

	track, err := pc.NewRTCSampleTrack(DefaultPayloadTypeH264, "trackId", "trackLabel")
	assert.Nil(t, err)

	// This channel should not be set up for a RTCSample track
	assert.Panics(t, func() {
		track.RawRTP <- &rtp.Packet{}
	})

	assert.NotPanics(t, func() {
		track.Samples <- media.RTCSample{}
	})
}

func TestRTCPeerConnection_EventHandlers(t *testing.T) {
	api := NewAPI()
	pc, err := api.New(RTCConfiguration{})
	assert.Nil(t, err)

	onTrackCalled := make(chan bool)
	onICEConnectionStateChangeCalled := make(chan bool)
	onDataChannelCalled := make(chan bool)

	// Verify that the noop case works
	assert.NotPanics(t, func() { pc.onTrack(nil) })
	assert.NotPanics(t, func() { pc.onICEConnectionStateChange(ice.ConnectionStateNew) })

	pc.OnTrack(func(t *RTCTrack) {
		onTrackCalled <- true
	})

	pc.OnICEConnectionStateChange(func(cs ice.ConnectionState) {
		onICEConnectionStateChangeCalled <- true
	})

	pc.OnDataChannel(func(dc *RTCDataChannel) {
		onDataChannelCalled <- true
	})

	// Verify that the handlers deal with nil inputs
	assert.NotPanics(t, func() { pc.onTrack(nil) })
	assert.NotPanics(t, func() { go pc.onDataChannelHandler(nil) })

	// Verify that the set handlers are called
	assert.NotPanics(t, func() { pc.onTrack(&RTCTrack{}) })
	assert.NotPanics(t, func() { pc.onICEConnectionStateChange(ice.ConnectionStateNew) })
	assert.NotPanics(t, func() { go pc.onDataChannelHandler(&RTCDataChannel{settingEngine: &api.settingEngine}) })

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
