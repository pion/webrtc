// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindPrimaryPayloadTypeForRTX(t *testing.T) {
	for _, test := range []struct {
		Name                string
		Needle              RTPCodecParameters
		Haystack            []RTPCodecParameters
		ResultIsRTX         bool
		ResultPrimaryExists bool
	}{
		{
			Name: "not RTX",
			Needle: RTPCodecParameters{
				PayloadType: 2,
				RTPCodecCapability: RTPCodecCapability{
					MimeType:    MimeTypeH264,
					ClockRate:   90000,
					SDPFmtpLine: "apt=2",
				},
			},
			Haystack: []RTPCodecParameters{
				{
					PayloadType: 1,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:  MimeTypeH264,
						ClockRate: 90000,
					},
				},
			},
			ResultIsRTX:         false,
			ResultPrimaryExists: false,
		},
		{
			Name: "incorrect fmtp",
			Needle: RTPCodecParameters{
				PayloadType: 2,
				RTPCodecCapability: RTPCodecCapability{
					MimeType:    MimeTypeRTX,
					ClockRate:   90000,
					SDPFmtpLine: "incorrect-fmtp",
				},
			},
			Haystack: []RTPCodecParameters{
				{
					PayloadType: 1,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:  MimeTypeH264,
						ClockRate: 90000,
					},
				},
			},
			ResultIsRTX:         true,
			ResultPrimaryExists: false,
		},
		{
			Name: "incomplete fmtp",
			Needle: RTPCodecParameters{
				PayloadType: 2,
				RTPCodecCapability: RTPCodecCapability{
					MimeType:    MimeTypeRTX,
					ClockRate:   90000,
					SDPFmtpLine: "apt=",
				},
			},
			Haystack: []RTPCodecParameters{
				{
					PayloadType: 1,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:  MimeTypeH264,
						ClockRate: 90000,
					},
				},
			},
			ResultIsRTX:         true,
			ResultPrimaryExists: false,
		},
		{
			Name: "primary payload type outside range (negative)",
			Needle: RTPCodecParameters{
				PayloadType: 2,
				RTPCodecCapability: RTPCodecCapability{
					MimeType:    MimeTypeRTX,
					ClockRate:   90000,
					SDPFmtpLine: "apt=-10",
				},
			},
			Haystack: []RTPCodecParameters{
				{
					PayloadType: 1,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:  MimeTypeH264,
						ClockRate: 90000,
					},
				},
			},
			ResultIsRTX:         true,
			ResultPrimaryExists: false,
		},
		{
			Name: "primary payload type outside range (high positive)",
			Needle: RTPCodecParameters{
				PayloadType: 2,
				RTPCodecCapability: RTPCodecCapability{
					MimeType:    MimeTypeRTX,
					ClockRate:   90000,
					SDPFmtpLine: "apt=1000",
				},
			},
			Haystack: []RTPCodecParameters{
				{
					PayloadType: 1,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:  MimeTypeH264,
						ClockRate: 90000,
					},
				},
			},
			ResultIsRTX:         true,
			ResultPrimaryExists: false,
		},
		{
			Name: "non-matching needle",
			Needle: RTPCodecParameters{
				PayloadType: 2,
				RTPCodecCapability: RTPCodecCapability{
					MimeType:    MimeTypeRTX,
					ClockRate:   90000,
					SDPFmtpLine: "apt=23",
				},
			},
			Haystack: []RTPCodecParameters{
				{
					PayloadType: 1,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:  MimeTypeH264,
						ClockRate: 90000,
					},
				},
			},
			ResultIsRTX:         true,
			ResultPrimaryExists: false,
		},
		{
			Name: "matching needle",
			Needle: RTPCodecParameters{
				PayloadType: 2,
				RTPCodecCapability: RTPCodecCapability{
					MimeType:    MimeTypeRTX,
					ClockRate:   90000,
					SDPFmtpLine: "apt=1",
				},
			},
			Haystack: []RTPCodecParameters{
				{
					PayloadType: 1,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:  MimeTypeH264,
						ClockRate: 90000,
					},
				},
			},
			ResultIsRTX:         true,
			ResultPrimaryExists: true,
		},
		{
			Name: "matching fmtp is a substring",
			Needle: RTPCodecParameters{
				PayloadType: 2,
				RTPCodecCapability: RTPCodecCapability{
					MimeType:    MimeTypeRTX,
					ClockRate:   90000,
					SDPFmtpLine: "apt=1;rtx-time:2000",
				},
			},
			Haystack: []RTPCodecParameters{
				{
					PayloadType: 1,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:  MimeTypeH264,
						ClockRate: 90000,
					},
				},
			},
			ResultIsRTX:         true,
			ResultPrimaryExists: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			isRTX, primaryExists := primaryPayloadTypeForRTXExists(test.Needle, test.Haystack)
			assert.Equal(t, test.ResultIsRTX, isRTX)
			assert.Equal(t, test.ResultPrimaryExists, primaryExists)
		})
	}
}

func TestFindFECPayloadType(t *testing.T) {
	for _, test := range []struct {
		Haystack          []RTPCodecParameters
		ResultPayloadType PayloadType
	}{
		{
			Haystack: []RTPCodecParameters{
				{
					PayloadType: 1,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:     MimeTypeFlexFEC03,
						ClockRate:    90000,
						Channels:     0,
						SDPFmtpLine:  "repair-window=10000000",
						RTCPFeedback: nil,
					},
				},
			},
			ResultPayloadType: 1,
		},
		{
			Haystack: []RTPCodecParameters{
				{
					PayloadType: 2,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:     MimeTypeFlexFEC,
						ClockRate:    90000,
						Channels:     0,
						SDPFmtpLine:  "repair-window=10000000",
						RTCPFeedback: nil,
					},
				},
				{
					PayloadType: 1,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:     MimeTypeFlexFEC03,
						ClockRate:    90000,
						Channels:     0,
						SDPFmtpLine:  "repair-window=10000000",
						RTCPFeedback: nil,
					},
				},
			},
			ResultPayloadType: 2,
		},
		{
			Haystack: []RTPCodecParameters{
				{
					PayloadType: 100,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:     MimeTypeH265,
						ClockRate:    90000,
						Channels:     0,
						SDPFmtpLine:  "",
						RTCPFeedback: nil,
					},
				},
				{
					PayloadType: 101,
					RTPCodecCapability: RTPCodecCapability{
						MimeType:     MimeTypeRTX,
						ClockRate:    90000,
						Channels:     0,
						SDPFmtpLine:  "apt=100",
						RTCPFeedback: nil,
					},
				},
			},
			ResultPayloadType: 0,
		},
	} {
		assert.Equal(t, test.ResultPayloadType, findFECPayloadType(test.Haystack))
	}
}

func TestFindRTXPayloadType(t *testing.T) {
	haystack := []RTPCodecParameters{
		{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
			PayloadType:        96,
		},
		{
			RTPCodecCapability: RTPCodecCapability{MimeTypeRTX, 90000, 0, "apt=96;rtx-time=3000", nil},
			PayloadType:        97,
		},
		{
			RTPCodecCapability: RTPCodecCapability{MimeTypeVP9, 90000, 0, "", nil},
			PayloadType:        98,
		},
		{
			RTPCodecCapability: RTPCodecCapability{MimeTypeRTX, 90000, 0, "rtx-time=3000;apt=98", nil},
			PayloadType:        99,
		},
		{
			RTPCodecCapability: RTPCodecCapability{MimeTypeH264, 90000, 0, "apt=100", nil},
			PayloadType:        101,
		},
	}

	// the fmtp line of an RTX codec can carry parameters other than apt,
	// rtx-time being the one given as an example by RFC 4588.
	assert.Equal(t, PayloadType(97), findRTXPayloadType(96, haystack))
	assert.Equal(t, PayloadType(99), findRTXPayloadType(98, haystack))

	// a non-RTX codec that happens to carry apt must not be matched.
	assert.Equal(t, PayloadType(0), findRTXPayloadType(100, haystack))

	// no RTX codec for this payload type.
	assert.Equal(t, PayloadType(0), findRTXPayloadType(123, haystack))
}
