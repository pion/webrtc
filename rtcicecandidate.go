package webrtc

import (
	"errors"
	"fmt"
	"net"

	"github.com/pions/webrtc/pkg/ice"
)

// RTCIceCandidate represents a ice candidate
type RTCIceCandidate struct {
	Foundation     string              `json:"foundation"`
	Priority       uint32              `json:"priority"`
	IP             string              `json:"ip"`
	Protocol       RTCIceProtocol      `json:"protocol"`
	Port           uint16              `json:"port"`
	Typ            RTCIceCandidateType `json:"type"`
	RelatedAddress string              `json:"relatedAddress"`
	RelatedPort    uint16              `json:"relatedPort"`
}

func newRTCIceCandidatesFromICE(iceCandidates []*ice.Candidate) ([]RTCIceCandidate, error) {
	candidates := []RTCIceCandidate{}

	for _, i := range iceCandidates {
		c, err := newRTCIceCandidateFromICE(i)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}

	return candidates, nil
}

func newRTCIceCandidateFromICE(i *ice.Candidate) (RTCIceCandidate, error) {
	typ, err := convertTypeFromICE(i.Type)
	if err != nil {
		return RTCIceCandidate{}, err
	}
	return RTCIceCandidate{
		Foundation:     "foundation",
		Priority:       uint32(i.Priority(i.Type.Preference(), uint16(1))), // TODO: component support
		IP:             i.IP.String(),
		Protocol:       newRTCIceProtocol(i.NetworkType.NetworkShort()),
		Port:           uint16(i.Port), // TODO store differently in ICE package
		Typ:            typ,
		RelatedAddress: "", // TODO
		RelatedPort:    0,  // TODO (parse & store port correctly in ICE package)
	}, nil
}

func (c RTCIceCandidate) toICE() (*ice.Candidate, error) {
	ip := net.ParseIP(c.IP)
	if ip == nil {
		return nil, errors.New("Failed to parse IP address")
	}

	switch c.Typ {
	case RTCIceCandidateTypeHost:
		return ice.NewCandidateHost(c.Protocol.String(), ip, int(c.Port))

	case RTCIceCandidateTypeSrflx:
		return ice.NewCandidateServerReflexive(c.Protocol.String(), ip, int(c.Port),
			c.RelatedAddress, int(c.RelatedPort))

	default:
		return nil, fmt.Errorf("Unknown candidate type: %s", c.Typ)
	}
}

func convertTypeFromICE(t ice.CandidateType) (RTCIceCandidateType, error) {
	switch t {
	case ice.CandidateTypeHost:
		return RTCIceCandidateTypeHost, nil
	case ice.CandidateTypeServerReflexive:
		return RTCIceCandidateTypeSrflx, nil
		// case ice.CandidateTypePeerReflexive:
		// 	return RTCIceCandidateTypePrflx, nil
		// case ice.CandidateTypeRelay:
		// 	return RTCIceCandidateTypeRelay, nil
	default:
		return RTCIceCandidateType(t), fmt.Errorf("Unknown ICE candidate type: %s", t)
	}
}
