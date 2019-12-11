package webrtc

import (
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
