package ice

import (
	"errors"
	"fmt"
	"net"

	"github.com/pion/ice"
)

// Candidate represents a ice candidate
type Candidate struct {
	Foundation     string        `json:"foundation"`
	Priority       uint32        `json:"priority"`
	IP             string        `json:"ip"`
	Protocol       Protocol      `json:"protocol"`
	Port           uint16        `json:"port"`
	Typ            CandidateType `json:"type"`
	Component      uint16        `json:"component"`
	RelatedAddress string        `json:"relatedAddress"`
	RelatedPort    uint16        `json:"relatedPort"`
}

// Conversion for package ice

func newCandidatesFromICE(iceCandidates []ice.Candidate) ([]Candidate, error) {
	candidates := []Candidate{}

	for _, i := range iceCandidates {
		c, err := newCandidateFromICE(i)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}

	return candidates, nil
}

func newCandidateFromICE(i ice.Candidate) (Candidate, error) {
	typ, err := convertTypeFromICE(i.Type())
	if err != nil {
		return Candidate{}, err
	}
	protocol, err := NewProtocol(i.NetworkType().NetworkShort())
	if err != nil {
		return Candidate{}, err
	}

	c := Candidate{
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

func (c Candidate) toICE() (ice.Candidate, error) {
	ip := net.ParseIP(c.IP)
	if ip == nil {
		return nil, errors.New("failed to parse IP address")
	}

	switch c.Typ {
	case CandidateTypeHost:
		return ice.NewCandidateHost(c.Protocol.String(), ip, int(c.Port), c.Component)
	case CandidateTypeSrflx:
		return ice.NewCandidateServerReflexive(c.Protocol.String(), ip, int(c.Port), c.Component,
			c.RelatedAddress, int(c.RelatedPort))
	case CandidateTypePrflx:
		return ice.NewCandidatePeerReflexive(c.Protocol.String(), ip, int(c.Port), c.Component,
			c.RelatedAddress, int(c.RelatedPort))
	case CandidateTypeRelay:
		return ice.NewCandidateRelay(c.Protocol.String(), ip, int(c.Port), c.Component,
			c.RelatedAddress, int(c.RelatedPort))
	default:
		return nil, fmt.Errorf("unknown candidate type: %s", c.Typ)
	}
}

func convertTypeFromICE(t ice.CandidateType) (CandidateType, error) {
	switch t {
	case ice.CandidateTypeHost:
		return CandidateTypeHost, nil
	case ice.CandidateTypeServerReflexive:
		return CandidateTypeSrflx, nil
	case ice.CandidateTypePeerReflexive:
		return CandidateTypePrflx, nil
	case ice.CandidateTypeRelay:
		return CandidateTypeRelay, nil
	default:
		return CandidateType(t), fmt.Errorf("unknown ICE candidate type: %s", t)
	}
}

func (c Candidate) String() string {
	ic, err := c.toICE()
	if err != nil {
		return fmt.Sprintf("%#v failed to convert to ICE: %s", c, err)
	}
	return ic.String()
}
