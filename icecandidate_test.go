// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/pion/ice/v4"
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

func TestNewIdentifiedICECandidateFromICE(t *testing.T) {
	config := ice.CandidateHostConfig{
		Network:    "udp",
		Address:    "::1",
		Port:       1234,
		Component:  1,
		Foundation: "foundation",
		Priority:   128,
	}
	ice, err := ice.NewCandidateHost(&config)
	assert.NoError(t, err)

	ct, err := newICECandidateFromICE(ice, "1", 2)
	assert.NoError(t, err)

	assert.Equal(t, "1", ct.SDPMid)
	assert.Equal(t, uint16(2), ct.SDPMLineIndex)
}

func TestNewIdentifiedICECandidatesFromICE(t *testing.T) {
	ic, err := ice.NewCandidateHost(&ice.CandidateHostConfig{
		Network:    "udp",
		Address:    "::1",
		Port:       1234,
		Component:  1,
		Foundation: "foundation",
		Priority:   128,
	})

	assert.NoError(t, err)

	candidates := []ice.Candidate{ic, ic, ic}

	sdpMid := "1"
	sdpMLineIndex := uint16(2)

	results, err := newICECandidatesFromICE(candidates, sdpMid, sdpMLineIndex)

	assert.NoError(t, err)

	assert.Equal(t, 3, len(results))

	for _, result := range results {
		assert.Equal(t, sdpMid, result.SDPMid)
		assert.Equal(t, sdpMLineIndex, result.SDPMLineIndex)
	}
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

func TestICECandidateZeroSDPid(t *testing.T) {
	candidate := ICECandidate{}

	assert.Equal(t, candidate.SDPMid, "")
	assert.Equal(t, candidate.SDPMLineIndex, uint16(0))
}

func TestICECandidateSDPMid_ToJSON(t *testing.T) {
	candidate := ICECandidate{}

	candidate.SDPMid = "0"
	candidate.SDPMLineIndex = 1

	assert.Equal(t, candidate.SDPMid, "0")
	assert.Equal(t, candidate.SDPMLineIndex, uint16(1))
}

func TestICECandidateExtensions_ToJSON(t *testing.T) {
	candidates := []struct {
		candidate  string
		extensions []ice.CandidateExtension
	}{
		{
			"2637185494 1 udp 2121932543 192.168.1.4 50723 typ host generation 1 ufrag Jzd0 network-id 1",
			[]ice.CandidateExtension{
				{
					Key:   "generation",
					Value: "1",
				},
				{
					Key:   "ufrag",
					Value: "Jzd0",
				},
				{
					Key:   "network-id",
					Value: "1",
				},
			},
		},
		{
			"1052353102 1 tcp 2128609279 192.168.0.196 0 typ host tcptype active ufrag Jzd0 network-id 1",
			[]ice.CandidateExtension{
				{
					Key:   "tcptype",
					Value: "active",
				},
				{
					Key:   "ufrag",
					Value: "Jzd0",
				},
				{
					Key:   "network-id",
					Value: "1",
				},
			},
		},
		{
			"1052353102 1 tcp 2128609279 192.168.0.196 0 typ host tcptype active ufrag Jzd0 network-id 1 empty-ext ",
			[]ice.CandidateExtension{
				{
					Key:   "tcptype",
					Value: "active",
				},
				{
					Key:   "ufrag",
					Value: "Jzd0",
				},
				{
					Key:   "network-id",
					Value: "1",
				},
				{
					Key:   "empty-ext",
					Value: "",
				},
			},
		},
		{
			"1052353102 1 tcp 2128609279 192.168.0.196 0 typ host tcptype active ufrag Jzd0 empty-ext  network-id 1",
			[]ice.CandidateExtension{
				{
					Key:   "tcptype",
					Value: "active",
				},
				{
					Key:   "ufrag",
					Value: "Jzd0",
				},
				{
					Key:   "empty-ext",
					Value: "",
				},
				{
					Key:   "network-id",
					Value: "1",
				},
			},
		},
	}

	for _, cand := range candidates {
		cand := cand
		candidate, err := ice.UnmarshalCandidate(cand.candidate)
		assert.NoError(t, err)

		sdpMid := "1"
		sdpMLineIndex := uint16(2)

		iceCandidate, err := newICECandidateFromICE(candidate, sdpMid, sdpMLineIndex)
		assert.NoError(t, err)

		candidateInit := iceCandidate.ToJSON()

		assert.Equal(t, sdpMLineIndex, *candidateInit.SDPMLineIndex)
		assert.Equal(t, "candidate:"+cand.candidate, candidateInit.Candidate)

		iceBack, err := iceCandidate.toICE()

		assert.NoError(t, err)
		assert.Equal(t, cand.extensions, iceBack.Extensions())
	}
}
