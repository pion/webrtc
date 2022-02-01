//go:build !js
// +build !js

package webrtc

import (
	"regexp"
	"testing"

	"github.com/pion/sdp/v3"
	"github.com/pion/transport/test"
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
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP9, 90000, 0, "profile-id=2", nil},
			PayloadType:        96,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{"video/rtx", 90000, 0, "apt=96", nil},
			PayloadType:        97,
		}, RTPCodecTypeVideo))
		assert.NoError(t, m.updateFromRemoteDescription(mustParse(profileLevels)))

		assert.True(t, m.negotiatedVideo)

		_, _, err := m.getCodecByPayload(97)
		assert.NoError(t, err)
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
