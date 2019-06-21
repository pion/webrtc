// +build !js

package webrtc

import (
	"testing"

	"github.com/pion/sdp/v2"
	"github.com/stretchr/testify/assert"
)

func TestICECandidateToSDP(t *testing.T) {
	testCases := []struct {
		native ICECandidate

		sdp sdp.ICECandidate
	}{
		{
			ICECandidate{
				Foundation: "foundation",
				Priority:   128,
				Address:    "1.0.0.1",
				Protocol:   ICEProtocolUDP,
				Port:       1234,
				Typ:        ICECandidateTypeHost,
				Component:  1,
			},

			sdp.ICECandidate{
				Foundation: "foundation",
				Priority:   128,
				Address:    "1.0.0.1",
				Protocol:   "udp",
				Port:       1234,
				Typ:        "host",
				Component:  1,
			},
		},
		{
			ICECandidate{
				Foundation:     "foundation",
				Priority:       128,
				Address:        "::1",
				Protocol:       ICEProtocolUDP,
				Port:           1234,
				Typ:            ICECandidateTypeSrflx,
				Component:      1,
				RelatedAddress: "1.0.0.1",
				RelatedPort:    4321,
			},

			sdp.ICECandidate{
				Foundation:     "foundation",
				Priority:       128,
				Address:        "::1",
				Protocol:       "udp",
				Port:           1234,
				Typ:            "srflx",
				Component:      1,
				RelatedAddress: "1.0.0.1",
				RelatedPort:    4321,
			},
		},
		{
			ICECandidate{
				Foundation:     "foundation",
				Priority:       128,
				Address:        "::1",
				Protocol:       ICEProtocolUDP,
				Port:           1234,
				Typ:            ICECandidateTypePrflx,
				Component:      1,
				RelatedAddress: "1.0.0.1",
				RelatedPort:    4321,
			},

			sdp.ICECandidate{
				Foundation:     "foundation",
				Priority:       128,
				Address:        "::1",
				Protocol:       "udp",
				Port:           1234,
				Typ:            "prflx",
				Component:      1,
				RelatedAddress: "1.0.0.1",
				RelatedPort:    4321,
			},
		},
	}

	for i, testCase := range testCases {
		actualSDP := iceCandidateToSDP(testCase.native)
		assert.Equal(t,
			testCase.sdp,
			actualSDP,
			"testCase: %d sdp not equal %v", i, actualSDP,
		)
	}
}
