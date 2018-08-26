package webrtc

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRTCIceServer_validate(t *testing.T) {
	assert.Nil(t, (RTCIceServer{
		URLs: []string{"turn:192.158.29.39?transport=udp"},
	}).validate())

	// pc, err := New(RTCConfiguration{})
	// assert.Nil(t, err)
	//
	// expected := RTCConfiguration{
	// 	IceServers:           []RTCIceServer{},
	// 	IceTransportPolicy:   RTCIceTransportPolicyAll,
	// 	BundlePolicy:         RTCBundlePolicyBalanced,
	// 	RtcpMuxPolicy:        RTCRtcpMuxPolicyRequire,
	// 	Certificates:         []RTCCertificate{},
	// 	IceCandidatePoolSize: 0,
	// }
	// actual := pc.GetConfiguration()
	// assert.True(t, &expected != &actual)
	// assert.Equal(t, expected.IceServers, actual.IceServers)
	// assert.Equal(t, expected.IceTransportPolicy, actual.IceTransportPolicy)
	// assert.Equal(t, expected.BundlePolicy, actual.BundlePolicy)
	// assert.Equal(t, expected.RtcpMuxPolicy, actual.RtcpMuxPolicy)
	// assert.NotEqual(t, len(expected.Certificates), len(actual.Certificates))
	// assert.Equal(t, expected.IceCandidatePoolSize, actual.IceCandidatePoolSize)
}
