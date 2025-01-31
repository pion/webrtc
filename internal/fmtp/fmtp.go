// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package fmtp implements per codec parsing of fmtp lines
package fmtp

import (
	"strings"
)

func parseParameters(line string) map[string]string {
	parameters := make(map[string]string)

	for _, p := range strings.Split(line, ";") {
		pp := strings.SplitN(strings.TrimSpace(p), "=", 2)
		key := strings.ToLower(pp[0])
		var value string
		if len(pp) > 1 {
			value = pp[1]
		}
		parameters[key] = value
	}

	return parameters
}

// ClockRateEqual checks whether two clock rates are equal.
func ClockRateEqual(mimeType string, valA, valB uint32) bool {
	// Clock rate and channel checks have been introduced quite recently.
	// Existing implementations often use VP8, H264 or Opus without setting clock rate or channels.
	// Keep compatibility with these situations.
	// It would be better to remove this exception in a future major release.
	switch {
	case strings.EqualFold(mimeType, "video/vp8"):
		if valA == 0 {
			valA = 90000
		}
		if valB == 0 {
			valB = 90000
		}

	case strings.EqualFold(mimeType, "audio/opus"):
		if valA == 0 {
			valA = 48000
		}
		if valB == 0 {
			valB = 48000
		}
	}

	return valA == valB
}

// ChannelsEqual checks whether two channels are equal.
func ChannelsEqual(mimeType string, valA, valB uint16) bool {
	// Clock rate and channel checks have been introduced quite recently.
	// Existing implementations often use VP8, H264 or Opus without setting clock rate or channels.
	// Keep compatibility with these situations.
	// It would be better to remove this exception in a future major release.
	if strings.EqualFold(mimeType, "audio/opus") {
		if valA == 0 {
			valA = 2
		}
		if valB == 0 {
			valB = 2
		}
	}

	if valA == 0 {
		valA = 1
	}
	if valB == 0 {
		valB = 1
	}

	return valA == valB
}

func paramsEqual(valA, valB map[string]string) bool {
	for k, v := range valA {
		if vb, ok := valB[k]; ok && !strings.EqualFold(vb, v) {
			return false
		}
	}

	for k, v := range valB {
		if va, ok := valA[k]; ok && !strings.EqualFold(va, v) {
			return false
		}
	}

	return true
}

// FMTP interface for implementing custom
// FMTP parsers based on MimeType.
type FMTP interface {
	// MimeType returns the MimeType associated with
	// the fmtp
	MimeType() string
	// Match compares two fmtp descriptions for
	// compatibility based on the MimeType
	Match(f FMTP) bool
	// Parameter returns a value for the associated key
	// if contained in the parsed fmtp string
	Parameter(key string) (string, bool)
}

// Parse parses an fmtp string based on the MimeType.
func Parse(mimeType string, clockRate uint32, channels uint16, line string) FMTP {
	var fmtp FMTP

	parameters := parseParameters(line)

	switch {
	// Clock rate and channel checks have been introduced quite recently.
	// Existing implementations often use VP8, H264 or Opus without setting clock rate or channels.
	// Keep compatibility with these situations.
	// It would be better to add a clock rate and channel check in a future major release.
	case strings.EqualFold(mimeType, "video/h264"):
		fmtp = &h264FMTP{
			parameters: parameters,
		}

	case strings.EqualFold(mimeType, "video/vp9") && clockRate == 90000 && channels == 0:
		fmtp = &vp9FMTP{
			parameters: parameters,
		}

	case strings.EqualFold(mimeType, "video/av1") && clockRate == 90000 && channels == 0:
		fmtp = &av1FMTP{
			parameters: parameters,
		}

	default:
		fmtp = &genericFMTP{
			mimeType:   mimeType,
			clockRate:  clockRate,
			channels:   channels,
			parameters: parameters,
		}
	}

	return fmtp
}

type genericFMTP struct {
	mimeType   string
	clockRate  uint32
	channels   uint16
	parameters map[string]string
}

func (g *genericFMTP) MimeType() string {
	return g.mimeType
}

// Match returns true if g and b are compatible fmtp descriptions
// The generic implementation is used for MimeTypes that are not defined.
func (g *genericFMTP) Match(b FMTP) bool {
	fmtp, ok := b.(*genericFMTP)
	if !ok {
		return false
	}

	return strings.EqualFold(g.mimeType, fmtp.MimeType()) &&
		ClockRateEqual(g.mimeType, g.clockRate, fmtp.clockRate) &&
		ChannelsEqual(g.mimeType, g.channels, fmtp.channels) &&
		paramsEqual(g.parameters, fmtp.parameters)
}

func (g *genericFMTP) Parameter(key string) (string, bool) {
	v, ok := g.parameters[key]

	return v, ok
}
