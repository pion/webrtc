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
func ICECandidateUnmarshal(raw string) (*ice.Candidate, error) {
	split := strings.Fields(raw)
	if len(split) < 8 {
		return nil, fmt.Errorf("attribute not long enough to be ICE candidate (%d) %s", len(split), raw)
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
		return nil, err
	}

	transport := split[2]

	// TODO verify valid address
	ip := net.ParseIP(split[4])
	if ip == nil {
		return nil, err
	}

	switch getValue("typ") {
	case "host":
		return ice.NewCandidateHost(transport, ip, port)
	case "srflx":
		return ice.NewCandidateServerReflexive(transport, ip, port, "", 0) // TODO: parse related address
	default:
		return nil, fmt.Errorf("Unhandled candidate typ %s", getValue("typ"))
	}
}

func iceCandidateString(c *ice.Candidate, component int) string {
	// TODO: calculate foundation
	switch c.Type {
	case ice.CandidateTypeHost:
		return fmt.Sprintf("foundation %d %s %d %s %d typ host generation 0",
			component, c.NetworkShort(), c.Priority(c.Type.Preference(), uint16(component)), c.IP, c.Port)

	case ice.CandidateTypeServerReflexive:
		return fmt.Sprintf("foundation %d %s %d %s %d typ srflx raddr %s rport %d generation 0",
			component, c.NetworkShort(), c.Priority(c.Type.Preference(), uint16(component)), c.IP, c.Port,
			c.RelatedAddress.Address, c.RelatedAddress.Port)
	}
	return ""
}

// ICECandidateMarshal takes a candidate and returns a string representation
func ICECandidateMarshal(c *ice.Candidate) []string {
	out := make([]string, 0)

	out = append(out, iceCandidateString(c, 1))
	out = append(out, iceCandidateString(c, 2))

	return out
}
