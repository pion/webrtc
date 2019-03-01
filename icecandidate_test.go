package webrtc

import (
	"net"
	"testing"

	"github.com/pions/sdp/v2"
	"github.com/pions/webrtc/internal/ice"
	"github.com/stretchr/testify/assert"
)

func TestICECandidate_Convert(t *testing.T) {
	testCases := []struct {
		native ICECandidate
		ice    *ice.Candidate
		sdp    sdp.ICECandidate
	}{
		{
			ICECandidate{
				Foundation: "foundation",
				Priority:   128,
				IP:         "1.0.0.1",
				Protocol:   ICEProtocolUDP,
				Port:       1234,
				Typ:        ICECandidateTypeHost,
				Component:  1,
			}, &ice.Candidate{
				IP:              net.ParseIP("1.0.0.1"),
				NetworkType:     ice.NetworkTypeUDP4,
				Port:            1234,
				Type:            ice.CandidateTypeHost,
				Component:       1,
				LocalPreference: 65535,
			},
			sdp.ICECandidate{
				Foundation: "foundation",
				Priority:   128,
				IP:         "1.0.0.1",
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
				IP:             "::1",
				Protocol:       ICEProtocolUDP,
				Port:           1234,
				Typ:            ICECandidateTypeSrflx,
				Component:      1,
				RelatedAddress: "1.0.0.1",
				RelatedPort:    4321,
			}, &ice.Candidate{
				IP:              net.ParseIP("::1"),
				NetworkType:     ice.NetworkTypeUDP6,
				Port:            1234,
				Type:            ice.CandidateTypeServerReflexive,
				Component:       1,
				LocalPreference: 65535,
				RelatedAddress: &ice.CandidateRelatedAddress{
					Address: "1.0.0.1",
					Port:    4321,
				},
			},
			sdp.ICECandidate{
				Foundation:     "foundation",
				Priority:       128,
				IP:             "::1",
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
				IP:             "::1",
				Protocol:       ICEProtocolUDP,
				Port:           1234,
				Typ:            ICECandidateTypePrflx,
				Component:      1,
				RelatedAddress: "1.0.0.1",
				RelatedPort:    4321,
			}, &ice.Candidate{
				IP:              net.ParseIP("::1"),
				NetworkType:     ice.NetworkTypeUDP6,
				Port:            1234,
				Type:            ice.CandidateTypePeerReflexive,
				Component:       1,
				LocalPreference: 65535,
				RelatedAddress: &ice.CandidateRelatedAddress{
					Address: "1.0.0.1",
					Port:    4321,
				},
			},
			sdp.ICECandidate{
				Foundation:     "foundation",
				Priority:       128,
				IP:             "::1",
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
		actualSDP := testCase.native.toSDP()
		assert.Equal(t,
			testCase.sdp,
			actualSDP,
			"testCase: %d sdp not equal %v", i, actualSDP,
		)
		actualICE, err := testCase.native.toICE()
		assert.Nil(t, err)
		assert.Equal(t,
			testCase.ice,
			actualICE,
			"testCase: %d ice not equal %v", i, actualSDP,
		)
	}
}

func TestConvertTypeFromICE(t *testing.T) {
	t.Run("host", func(t *testing.T) {
		ct, err := convertTypeFromICE(ice.CandidateTypeHost)
		if err != nil {
			t.Fatal("failed coverting ice.CandidateTypeHost")
		}
		if ct != ICECandidateTypeHost {
			t.Fatal("should be coverted to ICECandidateTypeHost")
		}
	})
	t.Run("srflx", func(t *testing.T) {
		ct, err := convertTypeFromICE(ice.CandidateTypeServerReflexive)
		if err != nil {
			t.Fatal("failed coverting ice.CandidateTypeServerReflexive")
		}
		if ct != ICECandidateTypeSrflx {
			t.Fatal("should be coverted to ICECandidateTypeSrflx")
		}
	})
	t.Run("prflx", func(t *testing.T) {
		ct, err := convertTypeFromICE(ice.CandidateTypePeerReflexive)
		if err != nil {
			t.Fatal("failed coverting ice.CandidateTypePeerReflexive")
		}
		if ct != ICECandidateTypePrflx {
			t.Fatal("should be coverted to ICECandidateTypePrflx")
		}
	})
}
