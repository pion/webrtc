package webrtc

import (
	"encoding/json"
	"syscall/js"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValueToICECandidate(t *testing.T) {
	testCases := []struct {
		jsonCandidate string
		expect        ICECandidate
	}{
		{
			// Firefox-style ICECandidateInit:
			`{"candidate":"1966762133 1 udp 2122260222 192.168.20.128 47298 typ srflx raddr 203.0.113.1 rport 5000"}`,
			ICECandidate{
				Foundation:     "1966762133",
				Priority:       2122260222,
				Address:        "192.168.20.128",
				Protocol:       ICEProtocolUDP,
				Port:           47298,
				Typ:            ICECandidateTypeSrflx,
				Component:      1,
				RelatedAddress: "203.0.113.1",
				RelatedPort:    5000,
			},
		}, {
			// Chrome/Webkit-style ICECandidate:
			`{"foundation":"1966762134", "component":"rtp", "protocol":"udp", "priority":2122260223, "address":"192.168.20.129", "port":47299, "type":"host", "relatedAddress":null}`,
			ICECandidate{
				Foundation:     "1966762134",
				Priority:       2122260223,
				Address:        "192.168.20.129",
				Protocol:       ICEProtocolUDP,
				Port:           47299,
				Typ:            ICECandidateTypeHost,
				Component:      1,
				RelatedAddress: "<null>",
				RelatedPort:    0,
			},
		}, {
			// Both are present, Chrome/Webkit-style takes precedent:
			`{"candidate":"1966762133 1 udp 2122260222 192.168.20.128 47298 typ srflx raddr 203.0.113.1 rport 5000", "foundation":"1966762134", "component":"rtp", "protocol":"udp", "priority":2122260223, "address":"192.168.20.129", "port":47299, "type":"host", "relatedAddress":null}`,
			ICECandidate{
				Foundation:     "1966762134",
				Priority:       2122260223,
				Address:        "192.168.20.129",
				Protocol:       ICEProtocolUDP,
				Port:           47299,
				Typ:            ICECandidateTypeHost,
				Component:      1,
				RelatedAddress: "<null>",
				RelatedPort:    0,
			},
		},
	}

	for i, testCase := range testCases {
		v := map[string]interface{}{}
		err := json.Unmarshal([]byte(testCase.jsonCandidate), &v)
		if err != nil {
			t.Errorf("Case %d: bad test, got error: %v", i, err)
		}
		assert.Equal(t, testCase.expect, *valueToICECandidate(js.ValueOf(v)))
	}
}
