package sdp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pions/webrtc/pkg/ice"
)

// ICECandidateUnmarshal takes a candidate strings and returns a ice.Candidate or nil if it fails to parse
func ICECandidateUnmarshal(raw string) ice.Candidate {
	split := strings.Fields(raw)
	if len(split) < 8 {
		fmt.Printf("Attribute not long enough to be ICE candidate (%d) %s \n", len(split), raw)
		return nil
	}

	getValue := func(key string) string {
		rtrnNext := false
		for _, i := range split {
			if rtrnNext {
				return i
			} else if i == key {
				rtrnNext = true
			}
		}
		return ""
	}

	port, err := strconv.Atoi(split[5])
	if err != nil {
		return nil
	}

	// TODO verify valid address
	address := split[4]

	switch getValue("typ") {
	case "host":
		return &ice.CandidateHost{
			CandidateBase: ice.CandidateBase{
				Protocol: ice.ProtoTypeUDP,
				Address:  address,
				Port:     port,
			},
		}
	case "srflx":
		return &ice.CandidateSrflx{
			CandidateBase: ice.CandidateBase{
				Protocol: ice.ProtoTypeUDP,
				Address:  address,
				Port:     port,
			},
		}
	default:
		return nil
	}
}

func iceSrflxCandidateString(c *ice.CandidateSrflx, component int) string {
	return fmt.Sprintf("udpcandidate %d udp %d %s %d typ srflx raddr %s rport %d generation 0",
		component, c.CandidateBase.Priority(ice.SrflxCandidatePreference, uint16(component)), c.CandidateBase.Address, c.CandidateBase.Port, c.RelatedAddress, c.RelatedPort)
}

func iceHostCandidateString(c *ice.CandidateHost, component int) string {
	return fmt.Sprintf("udpcandidate %d udp %d %s %d typ host generation 0",
		component, c.CandidateBase.Priority(ice.HostCandidatePreference, uint16(component)), c.CandidateBase.Address, c.CandidateBase.Port)
}

// ICECandidateMarshal takes a candidate and returns a string representation
func ICECandidateMarshal(c ice.Candidate) []string {
	out := make([]string, 0)

	switch c := c.(type) {
	case *ice.CandidateSrflx:
		out = append(out, iceSrflxCandidateString(c, 1))
		out = append(out, iceSrflxCandidateString(c, 2))
	case *ice.CandidateHost:
		out = append(out, iceHostCandidateString(c, 1))
		out = append(out, iceHostCandidateString(c, 2))
	}

	return out
}
