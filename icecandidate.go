package webrtc

import (
	"errors"
	"fmt"
	"net"

	"github.com/pion/ice"
)

// ICECandidate represents a ice candidate
type ICECandidate struct {
	Foundation     string           `json:"foundation"`
	Priority       uint32           `json:"priority"`
	IP             string           `json:"ip"`
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
		Foundation: "foundation",
		Priority:   i.Priority(),
		IP:         i.IP().String(),
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
	ip := net.ParseIP(c.IP)
	if ip == nil {
		return nil, errors.New("failed to parse IP address")
	}

	switch c.Typ {
	case ICECandidateTypeHost:
		return ice.NewCandidateHost(c.Protocol.String(), ip, int(c.Port), c.Component)
	case ICECandidateTypeSrflx:
		return ice.NewCandidateServerReflexive(c.Protocol.String(), ip, int(c.Port), c.Component,
			c.RelatedAddress, int(c.RelatedPort))
	case ICECandidateTypePrflx:
		return ice.NewCandidatePeerReflexive(c.Protocol.String(), ip, int(c.Port), c.Component,
			c.RelatedAddress, int(c.RelatedPort))
	case ICECandidateTypeRelay:
		return ice.NewCandidateRelay(c.Protocol.String(), ip, int(c.Port), c.Component,
			c.RelatedAddress, int(c.RelatedPort))
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
