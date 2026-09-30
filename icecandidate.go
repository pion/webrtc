// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"fmt"
	"strconv"

	"github.com/pion/ice/v5"
)

// ICECandidate represents a ice candidate.
type ICECandidate struct {
	statsID        string
	Foundation     string           `json:"foundation"`
	Priority       uint32           `json:"priority"`
	Address        string           `json:"address"`
	Protocol       ICEProtocol      `json:"protocol"`
	Port           uint16           `json:"port"`
	Typ            ICECandidateType `json:"type"`
	Component      uint16           `json:"component"`
	RelatedAddress string           `json:"relatedAddress"`
	RelatedPort    uint16           `json:"relatedPort"`
	TCPType        string           `json:"tcpType"`
	SDPMid         string           `json:"sdpMid"`
	SDPMLineIndex  uint16           `json:"sdpMLineIndex"`
	extensions     []ice.CandidateExtension
}

// Conversion for package ice.
func newICECandidatesFromICE(
	iceCandidates []ice.Candidate,
	sdpMid string,
	sdpMLineIndex uint16,
) ([]ICECandidate, error) {
	candidates := []ICECandidate{}

	for _, i := range iceCandidates {
		c, err := newICECandidateFromICE(i, sdpMid, sdpMLineIndex)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}

	return candidates, nil
}

func newICECandidateFromICE(candidate ice.Candidate, sdpMid string, sdpMLineIndex uint16) (ICECandidate, error) {
	typ, err := convertTypeFromICE(candidate.Type())
	if err != nil {
		return ICECandidate{}, err
	}
	protocol, err := NewICEProtocol(candidate.NetworkType().NetworkShort())
	if err != nil {
		return ICECandidate{}, err
	}

	newCandidate := ICECandidate{
		statsID:       candidate.ID(),
		Foundation:    candidate.Foundation(),
		Priority:      candidate.Priority(),
		Address:       candidate.Address(),
		Protocol:      protocol,
		Port:          uint16(candidate.Port()), //nolint:gosec // G115
		Component:     candidate.Component(),
		Typ:           typ,
		SDPMid:        sdpMid,
		SDPMLineIndex: sdpMLineIndex,
	}

	newCandidate.extensions = candidate.Extensions()
	if tcpType, ok := candidate.GetExtension("tcptype"); ok {
		newCandidate.TCPType = tcpType.Value
	}

	if address, ok := candidate.GetExtension("raddr"); ok {
		newCandidate.RelatedAddress = address.Value
	}
	if port, ok := candidate.GetExtension("rport"); ok {
		parsed, err := strconv.ParseUint(port.Value, 10, 16)
		if err == nil {
			newCandidate.RelatedPort = uint16(parsed)
		}
	}

	return newCandidate, nil
}

// ToICE converts ICECandidate to ice.Candidate.
func (c ICECandidate) ToICE() (cand ice.Candidate, err error) {
	candidateID := c.statsID
	extensions := c.extensions
	if extensions == nil {
		if c.RelatedAddress != "" {
			extensions = append(extensions,
				ice.CandidateExtension{Key: "raddr", Value: c.RelatedAddress},
				ice.CandidateExtension{Key: "rport", Value: strconv.Itoa(int(c.RelatedPort))},
			)
		}
		if c.TCPType != "" {
			extensions = append(extensions, ice.CandidateExtension{Key: "tcptype", Value: c.TCPType})
		}
	}
	switch c.Typ {
	case ICECandidateTypeHost:
		config := ice.CandidateHostConfig{
			CandidateID: candidateID,
			Extensions:  extensions,
			Network:     c.Protocol.String(),
			Address:     c.Address,
			Port:        int(c.Port),
			Component:   c.Component,
			Foundation:  c.Foundation,
			Priority:    c.Priority,
		}

		cand, err = ice.NewCandidateHost(&config)
	case ICECandidateTypeSrflx:
		config := ice.CandidateServerReflexiveConfig{
			CandidateID: candidateID,
			Extensions:  extensions,
			Network:     c.Protocol.String(),
			Address:     c.Address,
			Port:        int(c.Port),
			Component:   c.Component,
			Foundation:  c.Foundation,
			Priority:    c.Priority,
		}

		cand, err = ice.NewCandidateServerReflexive(&config)
	case ICECandidateTypePrflx:
		config := ice.CandidatePeerReflexiveConfig{
			CandidateID: candidateID,
			Extensions:  extensions,
			Network:     c.Protocol.String(),
			Address:     c.Address,
			Port:        int(c.Port),
			Component:   c.Component,
			Foundation:  c.Foundation,
			Priority:    c.Priority,
		}

		cand, err = ice.NewCandidatePeerReflexive(&config)
	case ICECandidateTypeRelay:
		config := ice.CandidateRelayConfig{
			CandidateID: candidateID,
			Extensions:  extensions,
			Network:     c.Protocol.String(),
			Address:     c.Address,
			Port:        int(c.Port),
			Component:   c.Component,
			Foundation:  c.Foundation,
			Priority:    c.Priority,
		}

		cand, err = ice.NewCandidateRelay(&config)
	default:
		return nil, fmt.Errorf("%w: %s", errICECandidateTypeUnknown, c.Typ)
	}

	return cand, err
}

func convertTypeFromICE(t ice.CandidateType) (ICECandidateType, error) {
	switch t {
	case ice.CandidateTypeHost:
		return ICECandidateTypeHost, nil
	case ice.CandidateTypeServerReflexive:
		return ICECandidateTypeSrflx, nil
	case ice.CandidateTypePeerReflexive:
		return ICECandidateTypePrflx, nil
	case ice.CandidateTypeRelay:
		return ICECandidateTypeRelay, nil
	default:
		return ICECandidateType(t), fmt.Errorf("%w: %s", errICECandidateTypeUnknown, t)
	}
}

func (c ICECandidate) String() string {
	ic, err := c.ToICE()
	if err != nil {
		return fmt.Sprintf("%#v failed to convert to ICE: %s", c, err)
	}

	return ic.String()
}

// ToJSON returns an ICECandidateInit
// as indicated by the spec https://w3c.github.io/webrtc-pc/#dom-rtcicecandidate-tojson
func (c ICECandidate) ToJSON() ICECandidateInit {
	candidateStr := ""

	candidate, err := c.ToICE()
	if err == nil {
		candidateStr = candidate.Marshal()
	}

	return ICECandidateInit{
		Candidate:     fmt.Sprintf("candidate:%s", candidateStr),
		SDPMid:        &c.SDPMid,
		SDPMLineIndex: &c.SDPMLineIndex,
	}
}
