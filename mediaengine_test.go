package webrtc

import (
	"testing"

	"github.com/pions/sdp/v2"
	"github.com/stretchr/testify/assert"
)

func TestCodecRegistration(t *testing.T) {
	api := NewAPI()
	const invalidPT = 255

	api.mediaEngine.RegisterDefaultCodecs()

	testCases := []struct {
		c uint8
		e error
	}{
		{DefaultPayloadTypeG722, nil},
		{DefaultPayloadTypeOpus, nil},
		{DefaultPayloadTypeVP8, nil},
		{DefaultPayloadTypeVP9, nil},
		{DefaultPayloadTypeH264, nil},
		{invalidPT, ErrCodecNotFound},
	}

	for _, f := range testCases {
		_, err := api.mediaEngine.getCodec(f.c)
		assert.Equal(t, f.e, err)

	}
	_, err := api.mediaEngine.getCodecSDP(sdp.Codec{PayloadType: invalidPT})
	assert.Equal(t, err, ErrCodecNotFound)
}
