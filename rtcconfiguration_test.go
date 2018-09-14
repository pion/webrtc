package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRTCConfiguration_getIceServers(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		expectedServerStr := "stun:stun.l.google.com:19302"
		cfg := RTCConfiguration{
			IceServers: []RTCIceServer{
				{
					URLs: []string{expectedServerStr},
				},
			},
		}

		parsedURLs, err := cfg.getIceServers()
		assert.Nil(t, err)
		assert.Equal(t, expectedServerStr, (*parsedURLs)[0].String())
	})

	t.Run("Failure", func(t *testing.T) {
		expectedServerStr := "stun.l.google.com:19302"
		cfg := RTCConfiguration{
			IceServers: []RTCIceServer{
				{
					URLs: []string{expectedServerStr},
				},
			},
		}

		_, err := cfg.getIceServers()
		assert.NotNil(t, err)
	})
}
