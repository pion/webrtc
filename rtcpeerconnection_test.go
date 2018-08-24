package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestRTCPeerConnection_initConfiguration(t *testing.T) {
	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)

	expected := InvalidAccessError{Err: ErrCertificateExpired}

	_, actualError := New(RTCConfiguration{
		Certificates: []RTCCertificate{
			NewRTCCertificate(pk, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
	})
	assert.EqualError(t, actualError, expected.Error())
}

func TestRTCPeerConnection_SetConfiguration(t *testing.T) {

}

func TestRTCPeerConnection_GetConfiguration(t *testing.T) {
	expected := RTCConfiguration{
		IceServers:           []RTCIceServer{},
		IceTransportPolicy:   RTCIceTransportPolicyAll,
		BundlePolicy:         RTCBundlePolicyBalanced,
		RtcpMuxPolicy:        RTCRtcpMuxPolicyRequire,
		Certificates:         []RTCCertificate{},
		IceCandidatePoolSize: 0,
	}

	pc, err := New(RTCConfiguration{})
	assert.Nil(t, err)

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
