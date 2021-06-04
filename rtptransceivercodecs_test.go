package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTPTransceiverCodecs(t *testing.T) {
	me := MediaEngine{}
	assert.NoError(t, me.RegisterDefaultCodecs())

	tr := RTPTransceiver{kind: RTPCodecTypeVideo, me: &me, codecs: me.videoCodecs}
	assert.EqualValues(t, me.videoCodecs, tr.Codecs())

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
		assert.Error(t, tr.SetCodecPreferences(testCase))
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
}
