package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultCodecs(t *testing.T) {

	RegisterDefaultCodecs()

	codecs := DefaultMediaEngine.getCodecsByKind(RTCRtpCodecTypeAudio)
	assert.Len(t, codecs, 2)
	for _, c := range codecs {
		assert.Equal(t, c.Type, RTCRtpCodecTypeAudio)
		if *c.PayloadType < 96 || *c.PayloadType > 127 {
			assert.Fail(t, "payload type outside dynamic range")
		}
	}

	codecs = DefaultMediaEngine.getCodecsByKind(RTCRtpCodecTypeVideo)
	assert.Len(t, codecs, 3)
	for _, c := range codecs {
		assert.Equal(t, c.Type, RTCRtpCodecTypeVideo)
		if *c.PayloadType < 96 || *c.PayloadType > 127 {
			assert.Fail(t, "payload type outside dynamic range")
		}
	}

}
