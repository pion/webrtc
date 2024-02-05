// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/pion/sdp/v3"
	"github.com/pion/transport/v2/test"
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

// pion/example-webrtc-applications#89
func TestVideoCase(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = pc.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	offer, err := pc.CreateOffer(nil)
	assert.NoError(t, err)

	assert.True(t, regexp.MustCompile(`(?m)^a=rtpmap:\d+ H264/90000`).MatchString(offer.SDP))
	assert.True(t, regexp.MustCompile(`(?m)^a=rtpmap:\d+ VP8/90000`).MatchString(offer.SDP))
	assert.True(t, regexp.MustCompile(`(?m)^a=rtpmap:\d+ VP9/90000`).MatchString(offer.SDP))
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

		opusCodec, _, err := m.getCodecByPayload(111)
		assert.NoError(t, err)
		assert.Equal(t, opusCodec.MimeType, MimeTypeOpus)
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

		_, _, err := m.getCodecByPayload(111)
		assert.Error(t, err)

		opusCodec, _, err := m.getCodecByPayload(112)
		assert.NoError(t, err)
		assert.Equal(t, opusCodec.MimeType, MimeTypeOpus)
	})

	t.Run("Ambiguous Payload Type", func(t *testing.T) {
		const opusSamePayload = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=audio 9 UDP/TLS/RTP/SAVPF 96
a=rtpmap:96 opus/48000/2
a=fmtp:96 minptime=10; useinbandfec=1
`

		m := MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(opusSamePayload)))

		assert.False(t, m.negotiatedVideo)
		assert.True(t, m.negotiatedAudio)

		opusCodec, _, err := m.getCodecByPayload(96)
		assert.NoError(t, err)
		assert.Equal(t, opusCodec.MimeType, MimeTypeOpus)
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

		opusCodec, _, err := m.getCodecByPayload(111)
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

		opusCodec, _, err := m.getCodecByPayload(111)
		assert.NoError(t, err)
		assert.Equal(t, opusCodec.MimeType, MimeTypeOpus)
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
		registerSimulcastHeaderExtensions(&m, RTPCodecTypeAudio)
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(headerExtensions)))

		assert.False(t, m.negotiatedVideo)
		assert.True(t, m.negotiatedAudio)

		absID, absAudioEnabled, absVideoEnabled := m.getHeaderExtensionID(RTPHeaderExtensionCapability{sdp.ABSSendTimeURI})
		assert.Equal(t, absID, 0)
		assert.False(t, absAudioEnabled)
		assert.False(t, absVideoEnabled)

		midID, midAudioEnabled, midVideoEnabled := m.getHeaderExtensionID(RTPHeaderExtensionCapability{sdp.SDESMidURI})
		assert.Equal(t, midID, 7)
		assert.True(t, midAudioEnabled)
		assert.False(t, midVideoEnabled)
	})

	t.Run("Prefers exact codec matches", func(t *testing.T) {
		const profileLevels = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=video 60323 UDP/TLS/RTP/SAVPF 96 98
a=rtpmap:96 H264/90000
a=fmtp:96 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640c1f
a=rtpmap:98 H264/90000
a=fmtp:98 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f
`
		m := MediaEngine{}
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f", nil},
			PayloadType:        127,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(profileLevels)))

		assert.True(t, m.negotiatedVideo)
		assert.False(t, m.negotiatedAudio)

		supportedH264, _, err := m.getCodecByPayload(98)
		assert.NoError(t, err)
		assert.Equal(t, supportedH264.MimeType, MimeTypeH264)

		_, _, err = m.getCodecByPayload(96)
		assert.Error(t, err)
	})

	t.Run("Does not match when fmtpline is set and does not match", func(t *testing.T) {
		const profileLevels = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=video 60323 UDP/TLS/RTP/SAVPF 96 98
a=rtpmap:96 H264/90000
a=fmtp:96 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640c1f
`
		m := MediaEngine{}
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f", nil},
			PayloadType:        127,
		}, RTPCodecTypeVideo))
		assert.Error(t, m.updateFromRemoteDescription(mustParse(profileLevels)))

		_, _, err := m.getCodecByPayload(96)
		assert.Error(t, err)
	})

	t.Run("Matches when fmtpline is not set in offer, but exists in mediaengine", func(t *testing.T) {
		const profileLevels = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=video 60323 UDP/TLS/RTP/SAVPF 96
a=rtpmap:96 VP9/90000
`
		m := MediaEngine{}
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP9, 90000, 0, "profile-id=0", nil},
			PayloadType:        98,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(profileLevels)))

		assert.True(t, m.negotiatedVideo)

		_, _, err := m.getCodecByPayload(96)
		assert.NoError(t, err)
	})

	t.Run("Matches when fmtpline exists in neither", func(t *testing.T) {
		const profileLevels = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=video 60323 UDP/TLS/RTP/SAVPF 96
a=rtpmap:96 VP8/90000
`
		m := MediaEngine{}
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        96,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(profileLevels)))

		assert.True(t, m.negotiatedVideo)

		_, _, err := m.getCodecByPayload(96)
		assert.NoError(t, err)
	})

	t.Run("Matches when rtx apt for exact match codec", func(t *testing.T) {
		const profileLevels = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=video 60323 UDP/TLS/RTP/SAVPF 94 95 106 107 108 109 96 97 
a=rtpmap:94 VP8/90000
a=rtpmap:95 rtx/90000
a=fmtp:95 apt=94
a=rtpmap:106 H264/90000
a=fmtp:106 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f
a=rtpmap:107 rtx/90000
a=fmtp:107 apt=106
a=rtpmap:108 H264/90000
a=fmtp:108 level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42001f
a=rtpmap:109 rtx/90000
a=fmtp:109 apt=108
a=rtpmap:96 VP9/90000
a=fmtp:96 profile-id=2
a=rtpmap:97 rtx/90000
a=fmtp:97 apt=96
`
		m := MediaEngine{}
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        96,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{"video/rtx", 90000, 0, "apt=96", nil},
			PayloadType:        97,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f", nil},
			PayloadType:        102,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{"video/rtx", 90000, 0, "apt=102", nil},
			PayloadType:        103,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42001f", nil},
			PayloadType:        104,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{"video/rtx", 90000, 0, "apt=104", nil},
			PayloadType:        105,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP9, 90000, 0, "profile-id=2", nil},
			PayloadType:        98,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{"video/rtx", 90000, 0, "apt=98", nil},
			PayloadType:        99,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(profileLevels)))

		assert.True(t, m.negotiatedVideo)

		vp9Codec, _, err := m.getCodecByPayload(96)
		assert.NoError(t, err)
		assert.Equal(t, vp9Codec.MimeType, MimeTypeVP9)
		vp9RTX, _, err := m.getCodecByPayload(97)
		assert.NoError(t, err)
		assert.Equal(t, vp9RTX.MimeType, "video/rtx")

		h264P1Codec, _, err := m.getCodecByPayload(106)
		assert.NoError(t, err)
		assert.Equal(t, h264P1Codec.MimeType, MimeTypeH264)
		assert.Equal(t, h264P1Codec.SDPFmtpLine, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f")
		h264P1RTX, _, err := m.getCodecByPayload(107)
		assert.NoError(t, err)
		assert.Equal(t, h264P1RTX.MimeType, "video/rtx")
		assert.Equal(t, h264P1RTX.SDPFmtpLine, "apt=106")

		h264P0Codec, _, err := m.getCodecByPayload(108)
		assert.NoError(t, err)
		assert.Equal(t, h264P0Codec.MimeType, MimeTypeH264)
		assert.Equal(t, h264P0Codec.SDPFmtpLine, "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42001f")
		h264P0RTX, _, err := m.getCodecByPayload(109)
		assert.NoError(t, err)
		assert.Equal(t, h264P0RTX.MimeType, "video/rtx")
		assert.Equal(t, h264P0RTX.SDPFmtpLine, "apt=108")
	})

	t.Run("Matches when rtx apt for partial match codec", func(t *testing.T) {
		const profileLevels = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=video 60323 UDP/TLS/RTP/SAVPF 94 96 97
a=rtpmap:94 VP8/90000
a=rtpmap:96 VP9/90000
a=fmtp:96 profile-id=2
a=rtpmap:97 rtx/90000
a=fmtp:97 apt=96
`
		m := MediaEngine{}
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        94,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP9, 90000, 0, "profile-id=1", nil},
			PayloadType:        96,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{"video/rtx", 90000, 0, "apt=96", nil},
			PayloadType:        97,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(profileLevels)))

		assert.True(t, m.negotiatedVideo)

		_, _, err := m.getCodecByPayload(97)
		assert.ErrorIs(t, err, ErrCodecNotFound)
	})
}

func TestMediaEngineHeaderExtensionDirection(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	registerCodec := func(m *MediaEngine) {
		assert.NoError(t, m.RegisterCodec(
			RTPCodecParameters{
				RTPCodecCapability: RTPCodecCapability{MimeTypeOpus, 48000, 0, "", nil},
				PayloadType:        111,
			}, RTPCodecTypeAudio))
	}

	t.Run("No Direction", func(t *testing.T) {
		m := &MediaEngine{}
		registerCodec(m)
		assert.NoError(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{"pion-header-test"}, RTPCodecTypeAudio))

		params := m.getRTPParametersByKind(RTPCodecTypeAudio, []RTPTransceiverDirection{RTPTransceiverDirectionRecvonly})

		assert.Equal(t, 1, len(params.HeaderExtensions))
	})

	t.Run("Same Direction", func(t *testing.T) {
		m := &MediaEngine{}
		registerCodec(m)
		assert.NoError(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{"pion-header-test"}, RTPCodecTypeAudio, RTPTransceiverDirectionRecvonly))

		params := m.getRTPParametersByKind(RTPCodecTypeAudio, []RTPTransceiverDirection{RTPTransceiverDirectionRecvonly})

		assert.Equal(t, 1, len(params.HeaderExtensions))
	})

	t.Run("Different Direction", func(t *testing.T) {
		m := &MediaEngine{}
		registerCodec(m)
		assert.NoError(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{"pion-header-test"}, RTPCodecTypeAudio, RTPTransceiverDirectionSendonly))

		params := m.getRTPParametersByKind(RTPCodecTypeAudio, []RTPTransceiverDirection{RTPTransceiverDirectionRecvonly})

		assert.Equal(t, 0, len(params.HeaderExtensions))
	})

	t.Run("Invalid Direction", func(t *testing.T) {
		m := &MediaEngine{}
		registerCodec(m)

		assert.ErrorIs(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{"pion-header-test"}, RTPCodecTypeAudio, RTPTransceiverDirectionSendrecv), ErrRegisterHeaderExtensionInvalidDirection)
		assert.ErrorIs(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{"pion-header-test"}, RTPCodecTypeAudio, RTPTransceiverDirectionInactive), ErrRegisterHeaderExtensionInvalidDirection)
		assert.ErrorIs(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{"pion-header-test"}, RTPCodecTypeAudio, RTPTransceiverDirection(0)), ErrRegisterHeaderExtensionInvalidDirection)
	})

	t.Run("Unique extmapid with different codec", func(t *testing.T) {
		m := &MediaEngine{}
		registerCodec(m)
		assert.NoError(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{"pion-header-test"}, RTPCodecTypeAudio))
		assert.NoError(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{"pion-header-test2"}, RTPCodecTypeVideo))

		audio := m.getRTPParametersByKind(RTPCodecTypeAudio, []RTPTransceiverDirection{RTPTransceiverDirectionRecvonly})
		video := m.getRTPParametersByKind(RTPCodecTypeVideo, []RTPTransceiverDirection{RTPTransceiverDirectionRecvonly})

		assert.Equal(t, 1, len(audio.HeaderExtensions))
		assert.Equal(t, 1, len(video.HeaderExtensions))
		assert.NotEqual(t, audio.HeaderExtensions[0].ID, video.HeaderExtensions[0].ID)
	})
}

// If a user attempts to register a codec twice we should just discard duplicate calls
func TestMediaEngineDoubleRegister(t *testing.T) {
	m := MediaEngine{}

	assert.NoError(t, m.RegisterCodec(
		RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeOpus, 48000, 0, "", nil},
			PayloadType:        111,
		}, RTPCodecTypeAudio))

	assert.NoError(t, m.RegisterCodec(
		RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeOpus, 48000, 0, "", nil},
			PayloadType:        111,
		}, RTPCodecTypeAudio))

	assert.Equal(t, len(m.audioCodecs), 1)
}

// The cloned MediaEngine instance should be able to update negotiated header extensions.
func TestUpdateHeaderExtenstionToClonedMediaEngine(t *testing.T) {
	src := MediaEngine{}

	assert.NoError(t, src.RegisterCodec(
		RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeTypeOpus, 48000, 0, "", nil},
			PayloadType:        111,
		}, RTPCodecTypeAudio))

	assert.NoError(t, src.RegisterHeaderExtension(RTPHeaderExtensionCapability{"test-extension"}, RTPCodecTypeAudio))

	validate := func(m *MediaEngine) {
		assert.NoError(t, m.updateHeaderExtension(2, "test-extension", RTPCodecTypeAudio))

		id, audioNegotiated, videoNegotiated := m.getHeaderExtensionID(RTPHeaderExtensionCapability{URI: "test-extension"})
		assert.Equal(t, 2, id)
		assert.True(t, audioNegotiated)
		assert.False(t, videoNegotiated)
	}

	validate(&src)
	validate(src.copy())
}

func TestExtensionIdCollision(t *testing.T) {
	mustParse := func(raw string) sdp.SessionDescription {
		s := sdp.SessionDescription{}
		assert.NoError(t, s.Unmarshal([]byte(raw)))
		return s
	}
	sdpSnippet := `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
m=audio 9 UDP/TLS/RTP/SAVPF 111
a=extmap:2 urn:ietf:params:rtp-hdrext:sdes:mid
a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level
a=extmap:5 urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id
a=rtpmap:111 opus/48000/2
`

	m := MediaEngine{}
	assert.NoError(t, m.RegisterDefaultCodecs())

	assert.NoError(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{sdp.SDESMidURI}, RTPCodecTypeVideo))
	assert.NoError(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{"urn:3gpp:video-orientation"}, RTPCodecTypeVideo))

	assert.NoError(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{sdp.SDESMidURI}, RTPCodecTypeAudio))
	assert.NoError(t, m.RegisterHeaderExtension(RTPHeaderExtensionCapability{sdp.AudioLevelURI}, RTPCodecTypeAudio))

	assert.NoError(t, m.updateFromRemoteDescription(mustParse(sdpSnippet)))

	assert.True(t, m.negotiatedAudio)
	assert.False(t, m.negotiatedVideo)

	id, audioNegotiated, videoNegotiated := m.getHeaderExtensionID(RTPHeaderExtensionCapability{sdp.ABSSendTimeURI})
	assert.Equal(t, id, 0)
	assert.False(t, audioNegotiated)
	assert.False(t, videoNegotiated)

	id, audioNegotiated, videoNegotiated = m.getHeaderExtensionID(RTPHeaderExtensionCapability{sdp.SDESMidURI})
	assert.Equal(t, id, 2)
	assert.True(t, audioNegotiated)
	assert.False(t, videoNegotiated)

	id, audioNegotiated, videoNegotiated = m.getHeaderExtensionID(RTPHeaderExtensionCapability{sdp.AudioLevelURI})
	assert.Equal(t, id, 1)
	assert.True(t, audioNegotiated)
	assert.False(t, videoNegotiated)

	params := m.getRTPParametersByKind(RTPCodecTypeVideo, []RTPTransceiverDirection{RTPTransceiverDirectionSendonly})
	extensions := params.HeaderExtensions

	assert.Equal(t, 2, len(extensions))

	midIndex := -1
	if extensions[0].URI == sdp.SDESMidURI {
		midIndex = 0
	} else if extensions[1].URI == sdp.SDESMidURI {
		midIndex = 1
	}

	voIndex := -1
	if extensions[0].URI == "urn:3gpp:video-orientation" {
		voIndex = 0
	} else if extensions[1].URI == "urn:3gpp:video-orientation" {
		voIndex = 1
	}

	assert.NotEqual(t, midIndex, -1)
	assert.NotEqual(t, voIndex, -1)

	assert.Equal(t, 2, extensions[midIndex].ID)
	assert.NotEqual(t, 1, extensions[voIndex].ID)
	assert.NotEqual(t, 2, extensions[voIndex].ID)
	assert.NotEqual(t, 5, extensions[voIndex].ID)
}

func TestCaseInsensitiveMimeType(t *testing.T) {
	const offerSdp = `
v=0
o=- 8448668841136641781 4 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE 0 1 2
a=extmap-allow-mixed
a=msid-semantic: WMS 4beea6b0-cf95-449c-a1ec-78e16b247426
m=video 9 UDP/TLS/RTP/SAVPF 96 127
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:1/MvHwjAyVf27aLu
a=ice-pwd:3dBU7cFOBl120v33cynDvN1E
a=ice-options:google-ice
a=fingerprint:sha-256 75:74:5A:A6:A4:E5:52:F4:A7:67:4C:01:C7:EE:91:3F:21:3D:A2:E3:53:7B:6F:30:86:F2:30:AA:65:FB:04:24
a=setup:actpass
a=mid:1
a=sendonly
a=rtpmap:96 VP8/90000
a=rtcp-fb:96 goog-remb
a=rtcp-fb:96 transport-cc
a=rtcp-fb:96 ccm fir
a=rtcp-fb:96 nack
a=rtcp-fb:96 nack pli
a=rtpmap:127 H264/90000
a=rtcp-fb:127 goog-remb
a=rtcp-fb:127 transport-cc
a=rtcp-fb:127 ccm fir
a=rtcp-fb:127 nack
a=rtcp-fb:127 nack pli
a=fmtp:127 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f

`

	for _, mimeTypeVp8 := range []string{
		"video/vp8",
		"video/VP8",
	} {
		t.Run(fmt.Sprintf("MimeType: %s", mimeTypeVp8), func(t *testing.T) {
			me := &MediaEngine{}
			feedback := []RTCPFeedback{
				{Type: TypeRTCPFBTransportCC},
				{Type: TypeRTCPFBCCM, Parameter: "fir"},
				{Type: TypeRTCPFBNACK},
				{Type: TypeRTCPFBNACK, Parameter: "pli"},
			}

			for _, codec := range []RTPCodecParameters{
				{
					RTPCodecCapability: RTPCodecCapability{MimeType: mimeTypeVp8, ClockRate: 90000, RTCPFeedback: feedback},
					PayloadType:        96,
				},
				{
					RTPCodecCapability: RTPCodecCapability{MimeType: "video/h264", ClockRate: 90000, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f", RTCPFeedback: feedback},
					PayloadType:        127,
				},
			} {
				assert.NoError(t, me.RegisterCodec(codec, RTPCodecTypeVideo))
			}

			api := NewAPI(WithMediaEngine(me))
			pc, err := api.NewPeerConnection(Configuration{
				SDPSemantics: SDPSemanticsUnifiedPlan,
			})
			assert.NoError(t, err)

			offer := SessionDescription{
				Type: SDPTypeOffer,
				SDP:  offerSdp,
			}

			assert.NoError(t, pc.SetRemoteDescription(offer))
			answer, err := pc.CreateAnswer(nil)
			assert.NoError(t, err)
			assert.NotNil(t, answer)
			assert.NoError(t, pc.SetLocalDescription(answer))
			assert.True(t, strings.Contains(answer.SDP, "VP8") || strings.Contains(answer.SDP, "vp8"))

			assert.NoError(t, pc.Close())
		})
	}
}
