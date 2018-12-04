package sdp

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/pions/webrtc/pkg/ice"
)

// ICECandidateUnmarshal takes a candidate strings and returns a ice.Candidate or nil if it fails to parse
// TODO: return error if parsing fails
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

	transport := split[2]

	// TODO verify valid address
	ip := net.ParseIP(split[4])
	if ip == nil {
		return nil
	}

	networkType := ice.DetermineNetworkType(transport, ip)

	switch getValue("typ") {
	case "host":
		return &ice.CandidateHost{
			CandidateBase: ice.CandidateBase{
				NetworkType: networkType,
				IP:          ip,
				Port:        port,
			},
		}
	case "srflx":
		return &ice.CandidateSrflx{
			CandidateBase: ice.CandidateBase{
				NetworkType: networkType,
				IP:          ip,
				Port:        port,
			},
		}
	default:
		return nil
	}
}

func iceSrflxCandidateString(c *ice.CandidateSrflx, component int) string {
	// TODO: calculate foundation
	return fmt.Sprintf("foundation %d %s %d %s %d typ srflx raddr %s rport %d generation 0",
		component, c.CandidateBase.NetworkShort(), c.CandidateBase.Priority(ice.SrflxCandidatePreference, uint16(component)), c.CandidateBase.IP, c.CandidateBase.Port, c.RelatedAddress, c.RelatedPort)
}

func iceHostCandidateString(c *ice.CandidateHost, component int) string {
	// TODO: calculate foundation
	return fmt.Sprintf("foundation %d %s %d %s %d typ host generation 0",
		component, c.CandidateBase.NetworkShort(), c.CandidateBase.Priority(ice.HostCandidatePreference, uint16(component)), c.CandidateBase.IP, c.CandidateBase.Port)
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
