package webrtc

import (
	"strings"

	"github.com/pion/ice"
)

// getICEServers side-steps the strict parsing mode of the ice package
// (as defined in https://tools.ietf.org/html/rfc7064) by stripping any
// erroneous queries from "stun(s):" URLs before parsing.
func (c Configuration) getICEServers() (*[]*ice.URL, error) {
	var iceServers []*ice.URL
	for _, server := range c.ICEServers {
		for _, rawURL := range server.URLs {
			if strings.HasPrefix(rawURL, "stun") {
				// strip the query from "stun(s):" if present
				parts := strings.Split(rawURL, "?")
				rawURL = parts[0]
			}
			url, err := ice.ParseURL(rawURL)
			if err != nil {
				return nil, err
			}
			iceServers = append(iceServers, url)
		}
	}
	return &iceServers, nil
}
