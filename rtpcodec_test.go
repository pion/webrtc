// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
