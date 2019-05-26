package webrtc

import (
	"net"
	"testing"

	"github.com/pion/ice"
	"github.com/stretchr/testify/assert"
)

func TestICECandidate_Convert(t *testing.T) {
	testCases := []struct {
		native ICECandidate

		expectedType           ice.CandidateType
		expectedNetwork        string
		expectedIP             net.IP
		expectedPort           int
		expectedComponent      uint16
		expectedRelatedAddress *ice.CandidateRelatedAddress
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
			},

			ice.CandidateTypeHost,
			"udp",
			net.ParseIP("1.0.0.1"),
			1234,
			1,
			nil,
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
			},

			ice.CandidateTypeServerReflexive,
			"udp",
			net.ParseIP("::1"),
			1234,
			1,
			&ice.CandidateRelatedAddress{
				Address: "1.0.0.1",
				Port:    4321,
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
			},

			ice.CandidateTypePeerReflexive,
			"udp",
			net.ParseIP("::1"),
			1234,
			1,
			&ice.CandidateRelatedAddress{
				Address: "1.0.0.1",
				Port:    4321,
			},
		},
	}

	for i, testCase := range testCases {
		actualICE, err := testCase.native.toICE()
		assert.Nil(t, err)

		var expectedICE ice.Candidate

		switch testCase.expectedType {
		case ice.CandidateTypeHost:
			expectedICE, err = ice.NewCandidateHost(testCase.expectedNetwork, testCase.expectedIP, testCase.expectedPort, testCase.expectedComponent)
		case ice.CandidateTypeServerReflexive:
			expectedICE, err = ice.NewCandidateServerReflexive(testCase.expectedNetwork, testCase.expectedIP, testCase.expectedPort, testCase.expectedComponent,
				testCase.expectedRelatedAddress.Address, testCase.expectedRelatedAddress.Port)
		case ice.CandidateTypePeerReflexive:
			expectedICE, err = ice.NewCandidatePeerReflexive(testCase.expectedNetwork, testCase.expectedIP, testCase.expectedPort, testCase.expectedComponent,
				testCase.expectedRelatedAddress.Address, testCase.expectedRelatedAddress.Port)
		}

		assert.Nil(t, err)
		assert.Equal(t, expectedICE, actualICE, "testCase: %d ice not equal %v", i, actualICE)
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
