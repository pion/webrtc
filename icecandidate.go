package webrtc

import (
	"fmt"

	"github.com/pion/ice"
	"github.com/pion/sdp/v2"
)

// ICECandidate represents a ice candidate
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
}

// Conversion for package ice

func newICECandidatesFromICE(iceCandidates []ice.Candidate) ([]ICECandidate, error) {
	candidates := []ICECandidate{}

	for _, i := range iceCandidates {
		c, err := newICECandidateFromICE(i)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}

	return candidates, nil
}

func newICECandidateFromICE(i ice.Candidate) (ICECandidate, error) {
	typ, err := convertTypeFromICE(i.Type())
	if err != nil {
		return ICECandidate{}, err
	}
	protocol, err := NewICEProtocol(i.NetworkType().NetworkShort())
	if err != nil {
		return ICECandidate{}, err
	}

	c := ICECandidate{
		statsID:    i.ID(),
		Foundation: "foundation",
		Priority:   i.Priority(),
		Address:    i.Address(),
		Protocol:   protocol,
		Port:       uint16(i.Port()),
		Component:  i.Component(),
		Typ:        typ,
	}

	if i.RelatedAddress() != nil {
		c.RelatedAddress = i.RelatedAddress().Address
		c.RelatedPort = uint16(i.RelatedAddress().Port)
	}

	return c, nil
}

func (c ICECandidate) toICE() (ice.Candidate, error) {
	candidateID := c.statsID
	switch c.Typ {
	case ICECandidateTypeHost:
		config := ice.CandidateHostConfig{
			CandidateID: candidateID,
			Network:     c.Protocol.String(),
			Address:     c.Address,
			Port:        int(c.Port),
			Component:   c.Component,
		}
		return ice.NewCandidateHost(&config)
	case ICECandidateTypeSrflx:
		config := ice.CandidateServerReflexiveConfig{
			CandidateID: candidateID,
			Network:     c.Protocol.String(),
			Address:     c.Address,
			Port:        int(c.Port),
			Component:   c.Component,
			RelAddr:     c.RelatedAddress,
			RelPort:     int(c.RelatedPort),
		}
		return ice.NewCandidateServerReflexive(&config)
	case ICECandidateTypePrflx:
		config := ice.CandidatePeerReflexiveConfig{
			CandidateID: candidateID,
			Network:     c.Protocol.String(),
			Address:     c.Address,
			Port:        int(c.Port),
			Component:   c.Component,
			RelAddr:     c.RelatedAddress,
			RelPort:     int(c.RelatedPort),
		}
		return ice.NewCandidatePeerReflexive(&config)
	case ICECandidateTypeRelay:
		config := ice.CandidateRelayConfig{
			CandidateID: candidateID,
			Network:     c.Protocol.String(),
			Address:     c.Address,
			Port:        int(c.Port),
			Component:   c.Component,
			RelAddr:     c.RelatedAddress,
			RelPort:     int(c.RelatedPort),
		}
		return ice.NewCandidateRelay(&config)
	default:
		return nil, fmt.Errorf("unknown candidate type: %s", c.Typ)
	}
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
		return ICECandidateType(t), fmt.Errorf("unknown ICE candidate type: %s", t)
	}
}

func (c ICECandidate) String() string {
	ic, err := c.toICE()
	if err != nil {
		return fmt.Sprintf("%#v failed to convert to ICE: %s", c, err)
	}
	return ic.String()
}

func iceCandidateToSDP(c ICECandidate) sdp.ICECandidate {
	return sdp.ICECandidate{
		Foundation:     c.Foundation,
		Priority:       c.Priority,
		Address:        c.Address,
		Protocol:       c.Protocol.String(),
		Port:           c.Port,
		Component:      c.Component,
		Typ:            c.Typ.String(),
		RelatedAddress: c.RelatedAddress,
		RelatedPort:    c.RelatedPort,
	}
}

// ToJSON returns an ICECandidateInit
// as indicated by the spec https://w3c.github.io/webrtc-pc/#dom-rtcicecandidate-tojson
func (c ICECandidate) ToJSON() ICECandidateInit {
	var sdpmLineIndex uint16
	return ICECandidateInit{
		Candidate:     fmt.Sprintf("candidate:%s", iceCandidateToSDP(c).Marshal()),
		SDPMLineIndex: &sdpmLineIndex,
	}
}
