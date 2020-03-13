// +build !js

package webrtc

import (
	"regexp"
	"testing"

	"github.com/pion/sdp/v2"
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
		{DefaultPayloadTypePCMU, nil},
		{DefaultPayloadTypePCMA, nil},
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

func TestPopulateFromSDP(t *testing.T) {
	const sdpValue = `v=0
o=- 884433216 1576829404 IN IP4 0.0.0.0
s=-
t=0 0
a=fingerprint:sha-256 1D:6B:6D:18:95:41:F9:BC:E4:AC:25:6A:26:A3:C8:09:D2:8C:EE:1B:7D:54:53:33:F7:E3:2C:0D:FE:7A:9D:6B
a=group:BUNDLE 0 1 2
m=audio 9 UDP/TLS/RTP/SAVPF 0 8 111 9
c=IN IP4 0.0.0.0
a=mid:0
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:111 opus/48000/2
a=fmtp:111 minptime=10;useinbandfec=1
a=rtpmap:9 G722/8000
a=ssrc:1823804162 cname:pion1
a=ssrc:1823804162 msid:pion1 audio
a=ssrc:1823804162 mslabel:pion1
a=ssrc:1823804162 label:audio
a=msid:pion1 audio
m=video 9 UDP/TLS/RTP/SAVPF 105 115 135
c=IN IP4 0.0.0.0
a=mid:1
a=rtpmap:105 VP8/90000
a=rtpmap:115 H264/90000
a=fmtp:115 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f
a=rtpmap:135 VP9/90000
a=ssrc:2949882636 cname:pion2
a=ssrc:2949882636 msid:pion2 video
a=ssrc:2949882636 mslabel:pion2
a=ssrc:2949882636 label:video
a=msid:pion2 video
m=application 9 DTLS/SCTP 5000
c=IN IP4 0.0.0.0
a=mid:2
a=sctpmap:5000 webrtc-datachannel 1024
`
	m := MediaEngine{}
	assertCodecWithPayloadType := func(name string, payloadType uint8) {
		for _, c := range m.codecs {
			if c.PayloadType == payloadType && c.Name == name {
				return
			}
		}
		t.Fatalf("Failed to find codec(%s) with PayloadType(%d)", name, payloadType)
	}

	m.RegisterDefaultCodecs()
	assert.NoError(t, m.PopulateFromSDP(SessionDescription{SDP: sdpValue}))

	assertCodecWithPayloadType(Opus, 111)
	assertCodecWithPayloadType(VP8, 105)
	assertCodecWithPayloadType(H264, 115)
	assertCodecWithPayloadType(VP9, 135)
}

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
