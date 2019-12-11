package webrtc

import (
	"strings"
)

// getICEServers side-steps the strict parsing mode of the ice package
// (as defined in https://tools.ietf.org/html/rfc7064) by stripping any
// erroneous queries from "stun(s):" URLs before parsing.
func (c Configuration) getICEServers() []ICEServer {
	iceServers := append([]ICEServer{}, c.ICEServers...)
	for _, server := range iceServers {
		for i, rawURL := range server.URLs {
			if strings.HasPrefix(rawURL, "stun") {
				// strip the query from "stun(s):" if present
				parts := strings.Split(rawURL, "?")
				rawURL = parts[0]
			}
			server.URLs[i] = rawURL
		}
	}
	return iceServers
}
