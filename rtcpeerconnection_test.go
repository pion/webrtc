package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRTCPeerConnection_initConfiguration(t *testing.T) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)

	certificate, err := GenerateCertificate(sk)
	assert.Nil(t, err)

	expected := InvalidAccessError{Err: ErrCertificateExpired}
	_, actualError := New(RTCConfiguration{
		Certificates: []RTCCertificate{*certificate},
	})
	assert.EqualError(t, actualError, expected.Error())
}

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

func ExampleNew() {
	if _, err := New(RTCConfiguration{}); err != nil {
		panic(err)
	}
}
