package sdp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pions/webrtc/pkg/ice"
)

// ICECandidateBuild takes a candidate strings and returns a ice.Candidate or nil if it fails to parse
func ICECandidateBuild(raw string) ice.Candidate {
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
				Protocol: ice.TransportUDP,
				Address:  address,
				Port:     port,
			},
		}
	default:
		return nil
	}
}
