//go:build opusred && !js

// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package opusredtest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSDP(opusPT, redPT uint8, hasRED bool) string {
	formats := fmt.Sprintf("%d", opusPT)
	extra := ""
	if hasRED {
		formats += fmt.Sprintf(" %d", redPT)
		extra = fmt.Sprintf("a=rtpmap:%d red/48000/2\r\na=fmtp:%d %d/%d\r\n", redPT, redPT, opusPT, opusPT)
	}
	return fmt.Sprintf("v=0\r\no=- 1 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\nm=audio 9 UDP/TLS/RTP/SAVPF %s\r\nc=IN IP4 0.0.0.0\r\na=mid:0\r\na=rtpmap:%d opus/48000/2\r\na=fmtp:%d minptime=10;useinbandfec=1\r\n%s", formats, opusPT, opusPT, extra)
}

func TestMappingNegotiationAndRemappedOffer(t *testing.T) {
	for _, pts := range [][2]uint8{{111, 63}, {109, 127}} {
		offer := testSDP(pts[0], pts[1], true)
		mapping, err := SelectOffer(offer)
		require.NoError(t, err)
		assert.Equal(t, pts[0], mapping.OpusPT)
		assert.Equal(t, pts[1], mapping.REDPT)
		assert.Equal(t, uint32(48000), mapping.ClockRate)
		assert.Equal(t, uint16(2), mapping.Channels)
		assert.Equal(t, "minptime=10;useinbandfec=1", mapping.OpusFMTP)
		assert.True(t, mapping.HasRED)
		accepted, err := ValidateAnswer(offer, offer)
		require.NoError(t, err)
		assert.Equal(t, mapping, accepted)
	}
}

func TestMappingRejectsInvalidNegotiation(t *testing.T) {
	offer := testSDP(111, 63, true)
	tests := map[string]string{
		"RED clock":                strings.ReplaceAll(offer, "red/48000/2", "red/16000/2"),
		"RED channels":             strings.ReplaceAll(offer, "red/48000/2", "red/48000/1"),
		"Opus clock":               strings.ReplaceAll(offer, "opus/48000/2", "opus/16000/2"),
		"Opus channels":            strings.ReplaceAll(offer, "opus/48000/2", "opus/48000/1"),
		"unoffered fmtp reference": strings.ReplaceAll(offer, "111/111", "112/112"),
		"mixed redundant codec":    strings.ReplaceAll(offer, "111/111", "111/0"),
		"missing RED fmtp":         strings.ReplaceAll(offer, "a=fmtp:63 111/111\r\n", ""),
		"missing Opus":             strings.ReplaceAll(offer, "opus/48000/2", "PCMU/8000/1"),
		"duplicate rtpmap":         offer + "a=rtpmap:63 red/48000/2\r\n",
		"duplicate fmtp":           offer + "a=fmtp:63 111/111\r\n",
		"unoffered attribute":      offer + "a=rtpmap:64 red/48000/2\r\n",
		"rejected audio":           strings.ReplaceAll(offer, "m=audio 9", "m=audio 0"),
		"out of range PT":          strings.ReplaceAll(offer, "63", "128"),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) { _, err := SelectOffer(raw); require.Error(t, err) })
	}
	for name, answer := range map[string]string{
		"Opus remap":  testSDP(109, 63, true),
		"RED remap":   testSDP(111, 127, true),
		"MID changed": strings.ReplaceAll(offer, "a=mid:0", "a=mid:1"),
	} {
		t.Run(name, func(t *testing.T) { _, err := ValidateAnswer(offer, answer); require.Error(t, err) })
	}
}

func TestMappingPlainOpusFallback(t *testing.T) {
	plain := testSDP(111, 63, false)
	mapping, err := ValidateAnswer(testSDP(111, 63, true), plain)
	require.NoError(t, err)
	assert.False(t, mapping.HasRED)
	assert.Zero(t, mapping.REDPT)
	_, err = ValidateAnswer(plain, testSDP(111, 63, true))
	require.Error(t, err)
}

func TestMappingRejectsIntroducedAudioPayloadTypes(t *testing.T) {
	offer := testSDP(111, 63, true)
	for _, test := range []struct {
		name   string
		answer string
	}{
		{name: "introduced static codec", answer: strings.Replace(offer, "SAVPF 111 63", "SAVPF 111 63 0", 1) + "a=rtpmap:0 PCMU/8000/1\r\n"},
		{name: "introduced dynamic codec", answer: strings.Replace(offer, "SAVPF 111 63", "SAVPF 111 63 112", 1) + "a=rtpmap:112 telephone-event/48000\r\n"},
	} {
		t.Run(test.name, func(t *testing.T) { _, err := ValidateAnswer(offer, test.answer); require.Error(t, err) })
	}
	offeredExtra := strings.Replace(offer, "SAVPF 111 63", "SAVPF 111 63 0", 1) + "a=rtpmap:0 PCMU/8000/1\r\n"
	changedExtra := strings.Replace(offeredExtra, "PCMU/8000/1", "PCMA/8000/1", 1)
	_, err := ValidateAnswer(offeredExtra, changedExtra)
	require.Error(t, err)
	_, err = ValidateAnswer(offeredExtra, offeredExtra)
	require.NoError(t, err)
}

func TestMappingRejectsAmbiguousOpusProfiles(t *testing.T) {
	offer := strings.Replace(testSDP(111, 63, true), "SAVPF 111 63", "SAVPF 111 63 112", 1) + "a=rtpmap:112 opus/48000/2\r\n"
	_, err := SelectOffer(offer)
	require.Error(t, err)
}
