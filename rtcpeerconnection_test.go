package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"github.com/stretchr/testify/assert"
	"testing"
)

// TODO
// func TestRTCPeerConnection_initConfiguration(t *testing.T) {
// 	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
// 	assert.Nil(t, err)
//
// 	certificate, err := GenerateCertificate(sk)
// 	assert.Nil(t, err)
//
// 	expected := InvalidAccessError{Err: ErrCertificateExpired}
// 	_, actualError := New(RTCConfiguration{
// 		Certificates: []RTCCertificate{*certificate},
// 	})
// 	assert.EqualError(t, actualError, expected.Error())
// }

func TestNew(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		secretKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		assert.Nil(t, err)

		certificate, err := GenerateCertificate(secretKey)
		assert.Nil(t, err)

		pc, err := New(RTCConfiguration{
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
			IceTransportPolicy: RTCIceTransportPolicyRelay,
			BundlePolicy: RTCBundlePolicyMaxCompat,
			RtcpMuxPolicy: RTCRtcpMuxPolicyNegotiate,
			IceCandidatePoolSize: 5,
			Certificates: []RTCCertificate{*certificate},
		})
		assert.Nil(t, err)
		assert.NotNil(t, pc)

		// testCases := []struct {
		// 	initializer func() (*RTCPeerConnection, error)
		// 	expectedErr error
		// }{
		// 	{func() (*RTCPeerConnection, error) {
		// 		secretKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		// 		assert.Nil(t, err)
		//
		// 		certificate, err := GenerateCertificate(secretKey)
		// 		assert.Nil(t, err)
		//
		// 		// expected := InvalidAccessError{Err: ErrCertificateExpired}
		// 		return New(RTCConfiguration{
		// 			Certificates: []RTCCertificate{*certificate},
		// 		})
		// 	}, &InvalidAccessError{ErrNoTurnCredencials}},
		// }
		//
		// 	for i, testCase := range testCases {
		// 	}

		// secretKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		// assert.Nil(t, err)
		//
		// certificate, err := GenerateCertificate(secretKey)
		// assert.Nil(t, err)
		//
		// expected := InvalidAccessError{ErrCertificateExpired}
		// _, actualError := New(RTCConfiguration{
		// 	Certificates: []RTCCertificate{*certificate},
		// })
		// assert.EqualError(t, actualError, expected.Error())
	})
	t.Run("Failure", func(t *testing.T) {
		testCases := []struct {
			iceServer   RTCIceServer
			expectedErr error
		}{
			{RTCIceServer{
				URLs: []string{"turn:192.158.29.39?transport=udp"},
			}, &InvalidAccessError{ErrNoTurnCredencials}},
		}

		for i, testCase := range testCases {


			assert.EqualError(t,
				testCase.iceServer.validate(),
				testCase.expectedErr.Error(),
				"testCase: %d %v", i, testCase,
			)
		}
	})
}

// TODO
// func TestRTCPeerConnection_SetConfiguration(t *testing.T) {
// 	pc, err := New(RTCConfiguration{})
// 	assert.Nil(t, err)
// 	pc.Close()
//
// 	expected := InvalidStateError{Err: ErrConnectionClosed}
// 	actualError := pc.SetConfiguration(RTCConfiguration{})
// 	assert.EqualError(t, actualError, expected.Error())
// }

func TestRTCPeerConnection_SetConfiguration_IsClosed(t *testing.T) {
	pc, err := New(RTCConfiguration{})
	assert.Nil(t, err)
	pc.Close()

	expected := InvalidStateError{Err: ErrConnectionClosed}
	actualError := pc.SetConfiguration(RTCConfiguration{})
	assert.EqualError(t, actualError, expected.Error())
}

func TestRTCPeerConnection_SetConfiguration_PeerIdentity(t *testing.T) {
	pc, err := New(RTCConfiguration{})
	assert.Nil(t, err)

	expected := InvalidModificationError{Err: ErrModifyingPeerIdentity}
	actualError := pc.SetConfiguration(RTCConfiguration{
		PeerIdentity: "unittest",
	})
	assert.EqualError(t, actualError, expected.Error())
}

func TestRTCPeerConnection_SetConfiguration_Certificates_Len(t *testing.T) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)

	certificate1, err := GenerateCertificate(sk)
	assert.Nil(t, err)

	certificate2, err := GenerateCertificate(sk)
	assert.Nil(t, err)

	pc, err := New(RTCConfiguration{})
	assert.Nil(t, err)

	expected := InvalidModificationError{Err: ErrModifyingCertificates}
	actualError := pc.SetConfiguration(RTCConfiguration{
		Certificates: []RTCCertificate{*certificate1, *certificate2},
	})
	assert.EqualError(t, actualError, expected.Error())
}

func TestRTCPeerConnection_SetConfiguration_Certificates_Equals(t *testing.T) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)

	certificate, err := GenerateCertificate(sk)
	assert.Nil(t, err)

	pc, err := New(RTCConfiguration{})

	expected := InvalidModificationError{Err: ErrModifyingCertificates}
	actualError := pc.SetConfiguration(RTCConfiguration{
		Certificates: []RTCCertificate{*certificate},
	})
	assert.EqualError(t, actualError, expected.Error())
}

func TestRTCPeerConnection_SetConfiguration_BundlePolicy(t *testing.T) {
	pc, err := New(RTCConfiguration{})
	assert.Nil(t, err)

	expected := InvalidModificationError{Err: ErrModifyingBundlePolicy}
	actualError := pc.SetConfiguration(RTCConfiguration{
		BundlePolicy: RTCBundlePolicyMaxCompat,
	})
	assert.EqualError(t, actualError, expected.Error())
}

func TestRTCPeerConnection_SetConfiguration_RtcpMuxPolicy(t *testing.T) {
	pc, err := New(RTCConfiguration{})
	assert.Nil(t, err)

	expected := InvalidModificationError{Err: ErrModifyingRtcpMuxPolicy}
	actualError := pc.SetConfiguration(RTCConfiguration{
		RtcpMuxPolicy: RTCRtcpMuxPolicyNegotiate,
	})
	assert.EqualError(t, actualError, expected.Error())
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

func ExampleNew_default() {
	if _, err := New(RTCConfiguration{}); err != nil {
		panic(err)
	}
}
