// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/pion/ice/v2"
	"github.com/stretchr/testify/assert"
)

func TestICECandidate_Convert(t *testing.T) {
	testCases := []struct {
		native ICECandidate

		expectedType           ice.CandidateType
		expectedNetwork        string
		expectedAddress        string
		expectedPort           int
		expectedComponent      uint16
		expectedRelatedAddress *ice.CandidateRelatedAddress
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

			ice.CandidateTypeHost,
			"udp",
			"1.0.0.1",
			1234,
			1,
			nil,
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

			ice.CandidateTypeServerReflexive,
			"udp",
			"::1",
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
				Address:        "::1",
				Protocol:       ICEProtocolUDP,
				Port:           1234,
				Typ:            ICECandidateTypePrflx,
				Component:      1,
				RelatedAddress: "1.0.0.1",
				RelatedPort:    4321,
			},

			ice.CandidateTypePeerReflexive,
			"udp",
			"::1",
			1234,
			1,
			&ice.CandidateRelatedAddress{
				Address: "1.0.0.1",
				Port:    4321,
			},
		},
	}

	for i, testCase := range testCases {
		var expectedICE ice.Candidate
		var err error
		switch testCase.expectedType { // nolint:exhaustive
		case ice.CandidateTypeHost:
			config := ice.CandidateHostConfig{
				Network:    testCase.expectedNetwork,
				Address:    testCase.expectedAddress,
				Port:       testCase.expectedPort,
				Component:  testCase.expectedComponent,
				Foundation: "foundation",
				Priority:   128,
			}
			expectedICE, err = ice.NewCandidateHost(&config)
		case ice.CandidateTypeServerReflexive:
			config := ice.CandidateServerReflexiveConfig{
				Network:    testCase.expectedNetwork,
				Address:    testCase.expectedAddress,
				Port:       testCase.expectedPort,
				Component:  testCase.expectedComponent,
				Foundation: "foundation",
				Priority:   128,
				RelAddr:    testCase.expectedRelatedAddress.Address,
				RelPort:    testCase.expectedRelatedAddress.Port,
			}
			expectedICE, err = ice.NewCandidateServerReflexive(&config)
		case ice.CandidateTypePeerReflexive:
			config := ice.CandidatePeerReflexiveConfig{
				Network:    testCase.expectedNetwork,
				Address:    testCase.expectedAddress,
				Port:       testCase.expectedPort,
				Component:  testCase.expectedComponent,
				Foundation: "foundation",
				Priority:   128,
				RelAddr:    testCase.expectedRelatedAddress.Address,
				RelPort:    testCase.expectedRelatedAddress.Port,
			}
			expectedICE, err = ice.NewCandidatePeerReflexive(&config)
		}
		assert.NoError(t, err)

		// first copy the candidate ID so it matches the new one
		testCase.native.statsID = expectedICE.ID()
		actualICE, err := testCase.native.toICE()
		assert.NoError(t, err)

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
			t.Fatal("should be converted to ICECandidateTypeHost")
		}
	})
	t.Run("srflx", func(t *testing.T) {
		ct, err := convertTypeFromICE(ice.CandidateTypeServerReflexive)
		if err != nil {
			t.Fatal("failed coverting ice.CandidateTypeServerReflexive")
		}
		if ct != ICECandidateTypeSrflx {
			t.Fatal("should be converted to ICECandidateTypeSrflx")
		}
	})
	t.Run("prflx", func(t *testing.T) {
		ct, err := convertTypeFromICE(ice.CandidateTypePeerReflexive)
		if err != nil {
			t.Fatal("failed coverting ice.CandidateTypePeerReflexive")
		}
		if ct != ICECandidateTypePrflx {
			t.Fatal("should be converted to ICECandidateTypePrflx")
		}
	})
}

func TestICECandidate_ToJSON(t *testing.T) {
	candidate := ICECandidate{
		Foundation: "foundation",
		Priority:   128,
		Address:    "1.0.0.1",
		Protocol:   ICEProtocolUDP,
		Port:       1234,
		Typ:        ICECandidateTypeHost,
		Component:  1,
	}

	candidateInit := candidate.ToJSON()

	assert.Equal(t, uint16(0), *candidateInit.SDPMLineIndex)
	assert.Equal(t, "candidate:foundation 1 udp 128 1.0.0.1 1234 typ host", candidateInit.Candidate)
}
