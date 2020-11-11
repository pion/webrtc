// +build !js

package webrtc

import (
	"regexp"
	"testing"

	"github.com/pion/sdp/v3"
	"github.com/stretchr/testify/assert"
)

// pion/webrtc#1078
func TestOpusCase(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = pc.AddTransceiverFromKind(RTPCodecTypeAudio)
	assert.NoError(t, err)

	offer, err := pc.CreateOffer(nil)
	assert.NoError(t, err)

	assert.True(t, regexp.MustCompile(`(?m)^a=rtpmap:\d+ opus/48000/2`).MatchString(offer.SDP))
	assert.NoError(t, pc.Close())
}

func TestMediaEngineRemoteDescription(t *testing.T) {
	mustParse := func(raw string) sdp.SessionDescription {
		s := sdp.SessionDescription{}
		assert.NoError(t, s.Unmarshal([]byte(raw)))
		return s
	}

	t.Run("No Media", func(t *testing.T) {
		const noMedia = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
`
		m := MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(noMedia)))

		assert.False(t, m.negotiatedVideo)
		assert.False(t, m.negotiatedAudio)
	})

	t.Run("Enable Opus", func(t *testing.T) {
		const opusSamePayload = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=audio 9 UDP/TLS/RTP/SAVPF 111
a=rtpmap:111 opus/48000/2
a=fmtp:111 minptime=10; useinbandfec=1
`

		m := MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(opusSamePayload)))

		assert.False(t, m.negotiatedVideo)
		assert.True(t, m.negotiatedAudio)

		opusCodec, err := m.getCodecByPayload(111)
		assert.NoError(t, err)
		assert.Equal(t, opusCodec.MimeType, mimeTypeOpus)
	})

	t.Run("Change Payload Type", func(t *testing.T) {
		const opusSamePayload = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=audio 9 UDP/TLS/RTP/SAVPF 112
a=rtpmap:112 opus/48000/2
a=fmtp:112 minptime=10; useinbandfec=1
`

		m := MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(opusSamePayload)))

		assert.False(t, m.negotiatedVideo)
		assert.True(t, m.negotiatedAudio)

		_, err := m.getCodecByPayload(111)
		assert.Error(t, err)

		opusCodec, err := m.getCodecByPayload(112)
		assert.NoError(t, err)
		assert.Equal(t, opusCodec.MimeType, mimeTypeOpus)
	})

	t.Run("Case Insensitive", func(t *testing.T) {
		const opusUpcase = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=audio 9 UDP/TLS/RTP/SAVPF 111
a=rtpmap:111 OPUS/48000/2
a=fmtp:111 minptime=10; useinbandfec=1
`

		m := MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(opusUpcase)))

		assert.False(t, m.negotiatedVideo)
		assert.True(t, m.negotiatedAudio)

		opusCodec, err := m.getCodecByPayload(111)
		assert.NoError(t, err)
		assert.Equal(t, opusCodec.MimeType, "audio/OPUS")
	})

	t.Run("Handle different fmtp", func(t *testing.T) {
		const opusNoFmtp = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=audio 9 UDP/TLS/RTP/SAVPF 111
a=rtpmap:111 opus/48000/2
`

		m := MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(opusNoFmtp)))

		assert.False(t, m.negotiatedVideo)
		assert.True(t, m.negotiatedAudio)

		opusCodec, err := m.getCodecByPayload(111)
		assert.NoError(t, err)
		assert.Equal(t, opusCodec.MimeType, mimeTypeOpus)
	})

	t.Run("Header Extensions", func(t *testing.T) {
		const headerExtensions = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=audio 9 UDP/TLS/RTP/SAVPF 111
a=extmap:7 urn:ietf:params:rtp-hdrext:sdes:mid
a=extmap:5 urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id
a=rtpmap:111 opus/48000/2
`

		m := MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(headerExtensions)))

		assert.False(t, m.negotiatedVideo)
		assert.True(t, m.negotiatedAudio)

		absID, absAudioEnabled, absVideoEnabled := m.GetHeaderExtensionID(RTPHeaderExtensionCapability{sdp.ABSSendTimeURI})
		assert.Equal(t, absID, 0)
		assert.False(t, absAudioEnabled)
		assert.False(t, absVideoEnabled)

		midID, midAudioEnabled, midVideoEnabled := m.GetHeaderExtensionID(RTPHeaderExtensionCapability{sdp.SDESMidURI})
		assert.Equal(t, midID, 7)
		assert.True(t, midAudioEnabled)
		assert.False(t, midVideoEnabled)
	})
}
