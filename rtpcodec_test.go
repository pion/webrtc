// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseREDFmtp(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected []PayloadType
		valid    bool
	}{
		{name: "two entries", line: "96/96", expected: []PayloadType{96, 96}, valid: true},
		{name: "more than two entries", line: "96/96/96", expected: []PayloadType{96, 96, 96}, valid: true},
		{name: "payload type zero", line: "0/0", expected: []PayloadType{0, 0}, valid: true},
		{name: "one entry", line: "96"},
		{name: "empty", line: ""},
		{name: "empty entry", line: "96/"},
		{name: "whitespace", line: "96 /96"},
		{name: "sign", line: "+96/+96"},
		{name: "non decimal", line: "0x60/0x60"},
		{name: "out of range", line: "128/128"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, valid := parseREDFmtp(test.line)
			assert.Equal(t, test.valid, valid)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestREDCodecAssociation(t *testing.T) {
	opus := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeOpus, ClockRate: 48000, Channels: 2},
		PayloadType:        96,
	}
	red := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeRED, ClockRate: 48000, Channels: 2, SDPFmtpLine: "96/96"},
		PayloadType:        97,
	}

	isRED, primary, attached := primaryPayloadTypeForRED(red, []RTPCodecParameters{red, opus})
	assert.True(t, isRED)
	assert.Equal(t, PayloadType(96), primary)
	assert.True(t, attached)
	assert.Equal(t, PayloadType(97), findREDPayloadType(opus.PayloadType, []RTPCodecParameters{red, opus}))

	for _, codec := range []RTPCodecParameters{
		{RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeRED, SDPFmtpLine: "96"}, PayloadType: 97},
		{RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeRED, SDPFmtpLine: "96/98"}, PayloadType: 97},
		{RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeRED, SDPFmtpLine: "98/98"}, PayloadType: 97},
	} {
		assert.Equal(t, PayloadType(0), findREDPayloadType(opus.PayloadType, []RTPCodecParameters{codec, opus}))
	}

	nonOpus := opus
	nonOpus.MimeType = MimeTypePCMU
	assert.Equal(t, PayloadType(0), findREDPayloadType(opus.PayloadType, []RTPCodecParameters{red, nonOpus}))
}

func TestFilterUnattachedRED(t *testing.T) {
	opus := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeOpus},
		PayloadType:        96,
	}
	attachedRED := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeRED, SDPFmtpLine: "96/96"},
		PayloadType:        97,
	}
	unattachedRED := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeRED, SDPFmtpLine: "98/98"},
		PayloadType:        99,
	}

	assert.Equal(t, []RTPCodecParameters{attachedRED, opus}, filterUnattachedRED(
		[]RTPCodecParameters{attachedRED, opus, unattachedRED},
	))
}

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
