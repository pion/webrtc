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
	testCodec := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{"video/testCodec", 90000, 0, "", nil},
		PayloadType:        50,
	}

	mediaEngine := &MediaEngine{}
	assert.NoError(t, mediaEngine.RegisterDefaultCodecs())

	offerPC, err := NewAPI(WithMediaEngine(mediaEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, mediaEngine.RegisterCodec(testCodec, RTPCodecTypeVideo))

	answerPC, err := NewAPI(WithMediaEngine(mediaEngine)).NewPeerConnection(Configuration{})
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

func Test_RTPTransceiver_SDP_Codec(t *testing.T) {
	tests := []struct {
		Label          string
		setPreferences bool
	}{
		{
			Label:          "NoSetCodecPreferences",
			setPreferences: false,
		},
		{
			Label:          "SetCodecPreferences",
			setPreferences: true,
		},
	}

	for _, test := range tests {
		t.Run(test.Label, func(t *testing.T) {
			pc, err := NewPeerConnection(Configuration{})
			assert.NoError(t, err)

			transceiver, err := pc.AddTransceiverFromKind(
				RTPCodecTypeVideo,
				RTPTransceiverInit{
					Direction: RTPTransceiverDirectionRecvonly,
				},
			)
			assert.NoError(t, err)

			if test.setPreferences {
				codec := RTPCodecCapability{
					"video/vp8", 90000, 0, "", nil,
				}

				err = transceiver.SetCodecPreferences(
					[]RTPCodecParameters{
						{
							RTPCodecCapability: codec,
						},
					},
				)
				assert.NoError(t, err)
			}

			offer, err := pc.CreateOffer(nil)
			assert.NoError(t, err)

			assert.Equal(t, true, strings.Contains(offer.SDP, "apt=96"))
			assert.NoError(t, pc.Close())
		})
	}
}
