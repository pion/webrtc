// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RTPTransceiver_SetCodecPreferences(t *testing.T) {
	mediaEngine := &MediaEngine{}
	api := NewAPI(WithMediaEngine(mediaEngine))
	assert.NoError(t, mediaEngine.RegisterDefaultCodecs())

	assert.NoError(t, mediaEngine.pushCodecs(mediaEngine.videoCodecs, RTPCodecTypeVideo))
	assert.NoError(t, mediaEngine.pushCodecs(mediaEngine.audioCodecs, RTPCodecTypeAudio))

	tr := RTPTransceiver{kind: RTPCodecTypeVideo, api: api, codecs: mediaEngine.videoCodecs}
	assert.EqualValues(t, mediaEngine.videoCodecs, tr.getCodecs())

	failTestCases := [][]RTPCodecParameters{
		{
			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeOpus, 48000, 2, "minptime=10;useinbandfec=1", nil},
				PayloadType:        111,
			},
		},
		{
			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
				PayloadType:        96,
			},
			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeOpus, 48000, 2, "minptime=10;useinbandfec=1", nil},
				PayloadType:        111,
			},
		},
	}

	for _, testCase := range failTestCases {
		assert.ErrorIs(t, tr.SetCodecPreferences(testCase), errRTPTransceiverCodecUnsupported)
	}

	successTestCases := [][]RTPCodecParameters{
		{
			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
				PayloadType:        96,
			},
		},
		{
			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
				PayloadType:        96,
			},
			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeRTX, 90000, 0, "apt=96", nil},
				PayloadType:        97,
			},

			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeVP9, 90000, 0, "profile-id=0", nil},
				PayloadType:        98,
			},
			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeRTX, 90000, 0, "apt=98", nil},
				PayloadType:        99,
			},
		},
	}

	for _, testCase := range successTestCases {
		assert.NoError(t, tr.SetCodecPreferences(testCase))
	}

	assert.NoError(t, tr.SetCodecPreferences(nil))
	assert.NotEqual(t, 0, len(tr.getCodecs()))

	assert.NoError(t, tr.SetCodecPreferences([]RTPCodecParameters{}))
	assert.NotEqual(t, 0, len(tr.getCodecs()))
}

// Assert that SetCodecPreferences properly filters codecs and PayloadTypes are respected.
func Test_RTPTransceiver_SetCodecPreferences_PayloadType(t *testing.T) {
	notOfferedCodec := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{"video/notOfferedCodec", 90000, 0, "", nil},
		PayloadType:        50,
	}
	offeredCodec := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{"video/offeredCodec", 90000, 0, "", nil},
		PayloadType:        52,
	}
	offeredCodecRTX := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{"video/rtx", 90000, 0, "apt=52", nil},
		PayloadType:        53,
	}

	mediaEngine := &MediaEngine{}
	assert.NoError(t, mediaEngine.RegisterDefaultCodecs())
	assert.NoError(t, mediaEngine.RegisterCodec(offeredCodec, RTPCodecTypeVideo))
	assert.NoError(t, mediaEngine.RegisterCodec(offeredCodecRTX, RTPCodecTypeVideo))

	offerPC, err := NewAPI(WithMediaEngine(mediaEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, mediaEngine.RegisterCodec(notOfferedCodec, RTPCodecTypeVideo))

	answerPC, err := NewAPI(WithMediaEngine(mediaEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = offerPC.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	track, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)
	answerTransceiver, err := answerPC.AddTransceiverFromTrack(
		track,
		RTPTransceiverInit{Direction: RTPTransceiverDirectionSendonly},
	)
	assert.NoError(t, err)

	assert.NoError(t, answerTransceiver.SetCodecPreferences([]RTPCodecParameters{
		notOfferedCodec,
		offeredCodec,
		offeredCodecRTX,
		{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        54,
		},
	}))

	offer, err := offerPC.CreateOffer(nil)
	assert.NoError(t, err)

	assert.NoError(t, offerPC.SetLocalDescription(offer))
	assert.NoError(t, answerPC.SetRemoteDescription(offer))

	answer, err := answerPC.CreateAnswer(nil)
	assert.NoError(t, err)

	// VP8 with proper PayloadType
	assert.NotEqual(t, -1, strings.Index(answer.SDP, "a=rtpmap:54 VP8/90000"))

	// testCodec1 and testCodec1RTX should be included as they are in the offer
	assert.NotEqual(t, -1, strings.Index(answer.SDP, "a=rtpmap:52 offeredCodec/90000"))
	assert.NotEqual(t, -1, strings.Index(answer.SDP, "a=rtpmap:53 rtx/90000"))
	assert.NotEqual(t, -1, strings.Index(answer.SDP, "a=fmtp:53 apt=52"))

	// testCodec is ignored since offerer doesn't support
	assert.Equal(t, -1, strings.Index(answer.SDP, "notOfferedCodec"))

	closePairNow(t, offerPC, answerPC)
}

// Assert that SetCodecPreferences and getCodecs properly filters unattached RTX.
func Test_RTPTransceiver_UnattachedRTX(t *testing.T) {
	testCodec := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{"video/testCodec", 90000, 0, "", nil},
		PayloadType:        50,
	}
	testCodecRTX := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{"video/rtx", 90000, 0, "apt=50", nil},
		PayloadType:        51,
	}

	mediaEngine := &MediaEngine{}
	assert.NoError(t, mediaEngine.RegisterDefaultCodecs())

	offerPC, err := NewAPI(WithMediaEngine(mediaEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, mediaEngine.RegisterCodec(testCodec, RTPCodecTypeVideo))
	assert.NoError(t, mediaEngine.RegisterCodec(testCodecRTX, RTPCodecTypeVideo))

	answerPC, err := NewAPI(WithMediaEngine(mediaEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = offerPC.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	answerTransceiver, err := answerPC.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	assert.NoError(t, answerTransceiver.SetCodecPreferences([]RTPCodecParameters{
		testCodecRTX,
		{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        52,
		},
	}))

	// rtx should not be in the list of transceiver codecs as testCodec (primary) is
	// not given to SetCodecPreferences
	answerTransceiver.mu.RLock()
	foundRTX := false
	for _, codec := range answerTransceiver.codecs {
		if strings.EqualFold(codec.RTPCodecCapability.MimeType, MimeTypeRTX) {
			foundRTX = true

			break
		}
	}
	assert.False(t, foundRTX)
	answerTransceiver.mu.RUnlock()

	assert.NoError(t, answerTransceiver.SetCodecPreferences([]RTPCodecParameters{
		testCodec,
		testCodecRTX,
		{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        52,
		},
	}))

	// rtx should be in the list of transceiver codecs as testCodec (primary) is
	// given to SetCodecPreferences
	answerTransceiver.mu.RLock()
	foundRTX = false
	for _, codec := range answerTransceiver.codecs {
		if strings.EqualFold(codec.RTPCodecCapability.MimeType, MimeTypeRTX) {
			foundRTX = true

			break
		}
	}
	assert.True(t, foundRTX)
	answerTransceiver.mu.RUnlock()

	// getCodecs() should have RTX as remote offer has not been processed
	codecs := answerTransceiver.getCodecs()
	foundRTX = false
	for _, codec := range codecs {
		if strings.EqualFold(codec.RTPCodecCapability.MimeType, MimeTypeRTX) {
			foundRTX = true

			break
		}
	}
	assert.True(t, foundRTX)

	offer, err := offerPC.CreateOffer(nil)
	assert.NoError(t, err)

	assert.NoError(t, offerPC.SetLocalDescription(offer))
	assert.NoError(t, answerPC.SetRemoteDescription(offer))

	// getCodecs() should filter out RTX as remote does not offer testCodec (primary)
	codecs = answerTransceiver.getCodecs()
	foundRTX = false
	for _, codec := range codecs {
		if strings.EqualFold(codec.RTPCodecCapability.MimeType, MimeTypeRTX) {
			foundRTX = true

			break
		}
	}
	assert.False(t, foundRTX)

	answer, err := answerPC.CreateAnswer(nil)
	assert.NoError(t, err)

	// VP8 with proper PayloadType
	assert.NotEqual(t, -1, strings.Index(answer.SDP, "a=rtpmap:52 VP8/90000"))

	// testCodec is ignored since offerer doesn't support
	assert.Equal(t, -1, strings.Index(answer.SDP, "testCodec"))
	assert.Equal(t, -1, strings.Index(answer.SDP, "rtx"))

	closePairNow(t, offerPC, answerPC)
}

func Test_ParseExtensionFromPaddingOnlyPacket(t *testing.T) {
	buf := []byte{
		176, 114, 15, 39, 0, 0, 0, 0, 209, 108, 221, 2, 190,
		222, 0, 2, 64, 49, 176, 102, 49, 0, 1, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255,
	}
	mid, rid, rsid, paddingOnly, err := handleUnknownRTPPacket(
		buf, uint8(4),
		uint8(10),
		uint8(11),
	)
	assert.NoError(t, err)

	assert.Equal(t, mid, "1")
	assert.Equal(t, rid, "")
	assert.Equal(t, rsid, "f")
	assert.Equal(t, paddingOnly, true)
}

// Use a literal offer so this regression cannot be hidden by the offerer's own
// payload mapping. H265 levels are compatible, but their payload identities and
// format parameters must still be preserved in the answer.
func TestRTPTransceiverH265AnswerPreservesPayloadTypes(t *testing.T) {
	for _, withRTX := range []bool{false, true} {
		t.Run(fmt.Sprintf("RTX=%t", withRTX), func(t *testing.T) {
			engine := &MediaEngine{}
			require.NoError(t, engine.RegisterCodec(RTPCodecParameters{
				RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeH265, ClockRate: 90000},
				PayloadType:        99,
			}, RTPCodecTypeVideo))
			if withRTX {
				require.NoError(t, engine.RegisterCodec(RTPCodecParameters{
					RTPCodecCapability: RTPCodecCapability{
						MimeType: MimeTypeRTX, ClockRate: 90000, SDPFmtpLine: "apt=99",
					},
					PayloadType: 100,
				}, RTPCodecTypeVideo))
			}
			pc, err := NewAPI(WithMediaEngine(engine)).NewPeerConnection(Configuration{})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, pc.Close()) })

			formats := "96 97"
			if withRTX {
				formats += " 98 99"
			}
			lines := strings.Split(fmt.Sprintf(`v=0
o=- 1234 1 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE 0
m=video 9 UDP/TLS/RTP/SAVPF %s
c=IN IP4 0.0.0.0
a=mid:0
a=sendonly
a=rtcp-mux
a=ice-ufrag:abcdefgh
a=ice-pwd:abcdefghijklmnopqrstuvwx
a=setup:actpass
a=fingerprint:sha-256 %s
a=rtpmap:96 H265/90000
a=fmtp:96 level-id=93;profile-id=1
a=rtpmap:97 H265/90000
a=fmtp:97 level-id=186;profile-id=1`, formats,
				strings.TrimSuffix(strings.Repeat("AA:", 32), ":")), "\n")
			if withRTX {
				lines = append(lines,
					"a=rtpmap:98 rtx/90000", "a=fmtp:98 apt=96",
					"a=rtpmap:99 rtx/90000", "a=fmtp:99 apt=97",
				)
			}
			offer := SessionDescription{Type: SDPTypeOffer, SDP: strings.Join(lines, "\r\n") + "\r\n"}
			require.NoError(t, pc.SetRemoteDescription(offer))
			answer, err := pc.CreateAnswer(nil)
			require.NoError(t, err)
			parsed, err := answer.Unmarshal()
			require.NoError(t, err)
			require.Len(t, parsed.MediaDescriptions, 1)
			require.ElementsMatch(t, strings.Fields(formats), parsed.MediaDescriptions[0].MediaName.Formats)
			require.Contains(t, answer.SDP, "a=fmtp:96 level-id=93;profile-id=1\r\n")
			require.Contains(t, answer.SDP, "a=fmtp:97 level-id=186;profile-id=1\r\n")
			if withRTX {
				require.Contains(t, answer.SDP, "a=fmtp:98 apt=96\r\n")
				require.Contains(t, answer.SDP, "a=fmtp:99 apt=97\r\n")
			}
		})
	}
}

func TestRTPTransceiverEquivalentCodecSelection(t *testing.T) {
	codec := func(payload PayloadType, format string) RTPCodecParameters {
		return RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{
				MimeType: MimeTypeH265, ClockRate: 90000, SDPFmtpLine: format,
			},
			PayloadType: payload,
		}
	}
	for _, tc := range []struct {
		name   string
		local  []RTPCodecParameters
		remote []RTPCodecParameters
		want   []PayloadType
	}{
		{
			name:   "preserve compatible payload identities",
			local:  []RTPCodecParameters{codec(96, "level-id=93"), codec(97, "level-id=186")},
			remote: []RTPCodecParameters{codec(96, "level-id=93"), codec(97, "level-id=186")},
			want:   []PayloadType{96, 97},
		},
		{
			name:   "remove selected codec when remapping is necessary",
			local:  []RTPCodecParameters{codec(96, "level-id=93"), codec(97, "level-id=186")},
			remote: []RTPCodecParameters{codec(112, "level-id=93"), codec(113, "level-id=186")},
			want:   []PayloadType{97, 96},
		},
		{
			name:   "preserve canonical remapping from earlier media section",
			local:  []RTPCodecParameters{codec(96, "level-id=93"), codec(97, "level-id=186")},
			remote: []RTPCodecParameters{codec(97, "level-id=186")},
			want:   []PayloadType{96},
		},
		{
			name:   "exact format outranks incompatible payload identity",
			local:  []RTPCodecParameters{codec(96, "profile-id=2"), codec(97, "profile-id=1")},
			remote: []RTPCodecParameters{codec(96, "profile-id=1")},
			want:   []PayloadType{97},
		},
		{
			name:   "preserve existing partial match fallback",
			local:  []RTPCodecParameters{codec(96, "profile-id=2")},
			remote: []RTPCodecParameters{codec(112, "profile-id=3")},
			want:   []PayloadType{96},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := &MediaEngine{}
			for _, local := range tc.local {
				require.NoError(t, engine.RegisterCodec(local, RTPCodecTypeVideo))
			}
			tr := &RTPTransceiver{kind: RTPCodecTypeVideo, api: NewAPI(WithMediaEngine(engine))}
			formats := []string{}
			attributes := []string{}
			for _, remote := range tc.remote {
				formats = append(formats, fmt.Sprint(remote.PayloadType))
				attributes = append(attributes,
					fmt.Sprintf("a=rtpmap:%d H265/90000", remote.PayloadType),
					fmt.Sprintf("a=fmtp:%d %s", remote.PayloadType, remote.SDPFmtpLine),
				)
			}
			header := strings.Split(fmt.Sprintf(`v=0
o=- 1234 1 IN IP4 127.0.0.1
s=-
t=0 0
m=video 9 UDP/TLS/RTP/SAVPF %s`, strings.Join(formats, " ")), "\n")
			offer := SessionDescription{
				Type: SDPTypeOffer,
				SDP:  strings.Join(append(header, attributes...), "\r\n") + "\r\n",
			}
			parsed, err := offer.Unmarshal()
			require.NoError(t, err)
			tr.setCodecPreferencesFromRemoteDescription(parsed.MediaDescriptions[0])
			got := tr.getCodecs()
			require.Len(t, got, len(tc.want))
			for i, payload := range tc.want {
				require.Equal(t, payload, got[i].PayloadType)
				require.Equal(t, tc.remote[i].SDPFmtpLine, got[i].SDPFmtpLine)
			}
		})
	}
}
