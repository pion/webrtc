package webrtc

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// func TestRTCPeerConnection_initConfiguration(t *testing.T) {
// 	expected := InvalidAccessError{Err: ErrCertificateExpired}
// 	_, actualError := New(RTCConfiguration{
// 		Certificates: []RTCCertificate{
// 			NewRTCCertificate(),
// 		},
// 	})
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

// func TestRTCPeerConnection_SetConfiguration_Certificates_Len(t *testing.T) {
// 	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
// 	assert.Nil(t, err)
//
// 	pc, err := New(RTCConfiguration{})
// 	assert.Nil(t, err)
//
// 	expected := InvalidModificationError{Err: ErrModifyingCertificates}
// 	actualError := pc.SetConfiguration(RTCConfiguration{
// 		Certificates: []RTCCertificate{
// 			NewRTCCertificate(pk, time.Time{}),
// 			NewRTCCertificate(pk, time.Time{}),
// 		},
// 	})
// 	assert.EqualError(t, actualError, expected.Error())
// }

// func TestRTCPeerConnection_SetConfiguration_Certificates_Equals(t *testing.T) {
// 	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
// 	assert.Nil(t, err)
//
// 	pc, err := New(RTCConfiguration{})
//
// 	skDER, err := x509.MarshalECPrivateKey(sk)
// 	assert.Nil(t, err)
// 	fmt.Printf("skDER: %x\n", skDER)
//
// 	skPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: skDER})
// 	fmt.Printf("skPEM: %v\n", string(skPEM))
//
// 	pkDER, err := x509.MarshalPKIXPublicKey(&sk.PublicKey)
// 	assert.Nil(t, err)
// 	fmt.Printf("pkDER: %x\n", pkDER)
//
// 	pkPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkDER})
// 	fmt.Printf("pkPEM: %v\n", string(pkPEM))
//
// 	expected := InvalidModificationError{Err: ErrModifyingCertificates}
// 	actualError := pc.SetConfiguration(RTCConfiguration{
// 		Certificates: []RTCCertificate{
// 			NewRTCCertificate(sk, time.Time{}),
// 		},
// 	})
// 	assert.EqualError(t, actualError, expected.Error())
// }

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

func ExampleNew() {
	if _, err := New(RTCConfiguration{}); err != nil {
		panic(err)
	}
}
