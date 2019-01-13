package webrtc

import (
	"net"
	"testing"

	"github.com/pions/sdp"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/stretchr/testify/assert"
)

func TestRTCIceCandidate_Convert(t *testing.T) {
	testCases := []struct {
		native RTCIceCandidate
		ice    *ice.Candidate
		sdp    sdp.ICECandidate
	}{
		{
			RTCIceCandidate{
				Foundation: "foundation",
				Priority:   128,
				IP:         "1.0.0.1",
				Protocol:   RTCIceProtocolUDP,
				Port:       1234,
				Typ:        RTCIceCandidateTypeHost,
			}, &ice.Candidate{
				IP:          net.ParseIP("1.0.0.1"),
				NetworkType: ice.NetworkTypeUDP4,
				Port:        1234,
				Type:        ice.CandidateTypeHost,
			},
			sdp.ICECandidate{
				Foundation: "foundation",
				Priority:   128,
				IP:         "1.0.0.1",
				Protocol:   "udp",
				Port:       1234,
				Typ:        "host",
			},
		},
		{
			RTCIceCandidate{
				Foundation:     "foundation",
				Priority:       128,
				IP:             "::1",
				Protocol:       RTCIceProtocolUDP,
				Port:           1234,
				Typ:            RTCIceCandidateTypeSrflx,
				RelatedAddress: "1.0.0.1",
				RelatedPort:    4321,
			}, &ice.Candidate{
				IP:          net.ParseIP("::1"),
				NetworkType: ice.NetworkTypeUDP6,
				Port:        1234,
				Type:        ice.CandidateTypeServerReflexive,
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
				RelatedAddress: "1.0.0.1",
				RelatedPort:    4321,
			},
		},
		{
			RTCIceCandidate{
				Foundation:     "foundation",
				Priority:       128,
				IP:             "::1",
				Protocol:       RTCIceProtocolUDP,
				Port:           1234,
				Typ:            RTCIceCandidateTypePrflx,
				RelatedAddress: "1.0.0.1",
				RelatedPort:    4321,
			}, &ice.Candidate{
				IP:          net.ParseIP("::1"),
				NetworkType: ice.NetworkTypeUDP6,
				Port:        1234,
				Type:        ice.CandidateTypePeerReflexive,
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
		if ct != RTCIceCandidateTypeHost {
			t.Fatal("should be coverted to RTCIceCandidateTypeHost")
		}
	})
	t.Run("srflx", func(t *testing.T) {
		ct, err := convertTypeFromICE(ice.CandidateTypeServerReflexive)
		if err != nil {
			t.Fatal("failed coverting ice.CandidateTypeServerReflexive")
		}
		if ct != RTCIceCandidateTypeSrflx {
			t.Fatal("should be coverted to RTCIceCandidateTypeSrflx")
		}
	})
	t.Run("prflx", func(t *testing.T) {
		ct, err := convertTypeFromICE(ice.CandidateTypePeerReflexive)
		if err != nil {
			t.Fatal("failed coverting ice.CandidateTypePeerReflexive")
		}
		if ct != RTCIceCandidateTypePrflx {
			t.Fatal("should be coverted to RTCIceCandidateTypePrflx")
		}
	})
}
