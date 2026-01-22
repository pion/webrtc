// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
