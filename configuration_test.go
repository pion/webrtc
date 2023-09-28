// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfiguration_getICEServers(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		expectedServerStr := "stun:stun.l.google.com:19302"
		cfg := Configuration{
			ICEServers: []ICEServer{
				{
					URLs: []string{expectedServerStr},
				},
			},
		}

		parsedURLs := cfg.getICEServers()
		assert.Equal(t, expectedServerStr, parsedURLs[0].URLs[0])
	})

	t.Run("Success", func(t *testing.T) {
		// ignore the fact that stun URLs shouldn't have a query
		serverStr := "stun:global.stun.twilio.com:3478?transport=udp"
		expectedServerStr := "stun:global.stun.twilio.com:3478"
		cfg := Configuration{
			ICEServers: []ICEServer{
				{
					URLs: []string{serverStr},
				},
			},
		}

		parsedURLs := cfg.getICEServers()
		assert.Equal(t, expectedServerStr, parsedURLs[0].URLs[0])
	})
}

func TestConfigurationJSON(t *testing.T) {
	j := `{
    "iceServers": [{"urls": ["turn:turn.example.org"],
                    "username": "jch",
                    "credential": "topsecret"
                  }],
    "iceTransportPolicy": "relay",
    "bundlePolicy": "balanced",
    "rtcpMuxPolicy": "require"
}`

	conf := Configuration{
		ICEServers: []ICEServer{
			{
				URLs:       []string{"turn:turn.example.org"},
				Username:   "jch",
				Credential: "topsecret",
			},
		},
		ICETransportPolicy: ICETransportPolicyRelay,
		BundlePolicy:       BundlePolicyBalanced,
		RTCPMuxPolicy:      RTCPMuxPolicyRequire,
	}

	var conf2 Configuration
	assert.NoError(t, json.Unmarshal([]byte(j), &conf2))
	assert.Equal(t, conf, conf2)

	j2, err := json.Marshal(conf2)
	assert.NoError(t, err)

	var conf3 Configuration
	assert.NoError(t, json.Unmarshal(j2, &conf3))
	assert.Equal(t, conf2, conf3)
}
