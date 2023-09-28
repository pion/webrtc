// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
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
	me := &MediaEngine{}
	api := NewAPI(WithMediaEngine(me))
	assert.NoError(t, me.RegisterDefaultCodecs())

	me.pushCodecs(me.videoCodecs, RTPCodecTypeVideo)
	me.pushCodecs(me.audioCodecs, RTPCodecTypeAudio)

	tr := RTPTransceiver{kind: RTPCodecTypeVideo, api: api, codecs: me.videoCodecs}
	assert.EqualValues(t, me.videoCodecs, tr.getCodecs())

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
				RTPCodecCapability: RTPCodecCapability{"video/rtx", 90000, 0, "apt=96", nil},
				PayloadType:        97,
			},

			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeVP9, 90000, 0, "profile-id=0", nil},
				PayloadType:        98,
			},
			{
				RTPCodecCapability: RTPCodecCapability{"video/rtx", 90000, 0, "apt=98", nil},
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

// Assert that SetCodecPreferences properly filters codecs and PayloadTypes are respected
func Test_RTPTransceiver_SetCodecPreferences_PayloadType(t *testing.T) {
	testCodec := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{"video/testCodec", 90000, 0, "", nil},
		PayloadType:        50,
	}

	m := &MediaEngine{}
	assert.NoError(t, m.RegisterDefaultCodecs())

	offerPC, err := NewAPI(WithMediaEngine(m)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, m.RegisterCodec(testCodec, RTPCodecTypeVideo))

	answerPC, err := NewAPI(WithMediaEngine(m)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = offerPC.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	answerTransceiver, err := answerPC.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	assert.NoError(t, answerTransceiver.SetCodecPreferences([]RTPCodecParameters{
		testCodec,
		{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        51,
		},
	}))

	offer, err := offerPC.CreateOffer(nil)
	assert.NoError(t, err)

	assert.NoError(t, offerPC.SetLocalDescription(offer))
	assert.NoError(t, answerPC.SetRemoteDescription(offer))

	answer, err := answerPC.CreateAnswer(nil)
	assert.NoError(t, err)

	// VP8 with proper PayloadType
	assert.NotEqual(t, -1, strings.Index(answer.SDP, "a=rtpmap:51 VP8/90000"))

	// testCodec is ignored since offerer doesn't support
	assert.Equal(t, -1, strings.Index(answer.SDP, "testCodec"))

	closePairNow(t, offerPC, answerPC)
}
