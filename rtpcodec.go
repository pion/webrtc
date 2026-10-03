// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/webrtc/v4/internal/fmtp"
)

// RTPCodecType determines the type of a codec.
type RTPCodecType int

const (
	// RTPCodecTypeUnknown is the enum's zero-value.
	RTPCodecTypeUnknown RTPCodecType = iota

	// RTPCodecTypeAudio indicates this is an audio codec.
	RTPCodecTypeAudio

	// RTPCodecTypeVideo indicates this is a video codec.
	RTPCodecTypeVideo
)

func (t RTPCodecType) String() string {
	switch t {
	case RTPCodecTypeAudio:
		return "audio" //nolint: goconst
	case RTPCodecTypeVideo:
		return "video" //nolint: goconst
	default:
		return ErrUnknownType.Error()
	}
}

// NewRTPCodecType creates a RTPCodecType from a string.
func NewRTPCodecType(r string) RTPCodecType {
	switch {
	case strings.EqualFold(r, RTPCodecTypeAudio.String()):
		return RTPCodecTypeAudio
	case strings.EqualFold(r, RTPCodecTypeVideo.String()):
		return RTPCodecTypeVideo
	default:
		return RTPCodecType(0)
	}
}

// RTPCodecCapability provides information about codec capabilities.
//
// https://w3c.github.io/webrtc-pc/#dictionary-rtcrtpcodeccapability-members
type RTPCodecCapability struct {
	MimeType     string
	ClockRate    uint32
	Channels     uint16
	SDPFmtpLine  string
	RTCPFeedback []RTCPFeedback
}

// RTPHeaderExtensionCapability is used to define a RFC5285 RTP header extension supported by the codec.
//
// https://w3c.github.io/webrtc-pc/#dom-rtcrtpcapabilities-headerextensions
type RTPHeaderExtensionCapability struct {
	URI string
}

// RTPHeaderExtensionParameter represents a negotiated RFC5285 RTP header extension.
//
// https://w3c.github.io/webrtc-pc/#dictionary-rtcrtpheaderextensionparameters-members
type RTPHeaderExtensionParameter struct {
	URI string
	ID  int
}

// RTPCodecParameters is a sequence containing the media codecs that an RtpSender
// will choose from, as well as entries for RTX, RED and FEC mechanisms. This also
// includes the PayloadType that has been negotiated
//
// https://w3c.github.io/webrtc-pc/#rtcrtpcodecparameters
type RTPCodecParameters struct {
	RTPCodecCapability
	PayloadType PayloadType

	statsID string
}

// RTPParameters is a list of negotiated codecs and header extensions
//
// https://w3c.github.io/webrtc-pc/#dictionary-rtcrtpparameters-members
type RTPParameters struct {
	HeaderExtensions []RTPHeaderExtensionParameter
	Codecs           []RTPCodecParameters
}

type codecMatchType int

const (
	codecMatchNone    codecMatchType = 0
	codecMatchPartial codecMatchType = 1
	codecMatchExact   codecMatchType = 2
)

// Do a fuzzy find for a codec in the list of codecs
// Used for lookup up a codec in an existing list to find a match
// Returns codecMatchExact, codecMatchPartial, or codecMatchNone.
func codecParametersFuzzySearch(
	needle RTPCodecParameters,
	haystack []RTPCodecParameters,
) (RTPCodecParameters, codecMatchType) {
	needleFmtp := fmtp.Parse(
		needle.RTPCodecCapability.MimeType,
		needle.RTPCodecCapability.ClockRate,
		needle.RTPCodecCapability.Channels,
		needle.RTPCodecCapability.SDPFmtpLine)

	// First attempt to match on MimeType + ClockRate + Channels + SDPFmtpLine
	for _, c := range haystack {
		cfmtp := fmtp.Parse(
			c.RTPCodecCapability.MimeType,
			c.RTPCodecCapability.ClockRate,
			c.RTPCodecCapability.Channels,
			c.RTPCodecCapability.SDPFmtpLine)

		if needleFmtp.Match(cfmtp) {
			return c, codecMatchExact
		}
	}

	// Fallback to just MimeType + ClockRate + Channels
	for _, c := range haystack {
		if strings.EqualFold(c.RTPCodecCapability.MimeType, needle.RTPCodecCapability.MimeType) &&
			fmtp.ClockRateEqual(c.RTPCodecCapability.MimeType,
				c.RTPCodecCapability.ClockRate,
				needle.RTPCodecCapability.ClockRate) &&
			fmtp.ChannelsEqual(c.RTPCodecCapability.MimeType,
				c.RTPCodecCapability.Channels,
				needle.RTPCodecCapability.Channels) {
			return c, codecMatchPartial
		}
	}

	return RTPCodecParameters{}, codecMatchNone
}

// Given a CodecParameters find the RTX CodecParameters if one exists.
func findRTXPayloadType(needle PayloadType, haystack []RTPCodecParameters) PayloadType {
	aptStr := fmt.Sprintf("apt=%d", needle)
	for _, c := range haystack {
		if aptStr == c.SDPFmtpLine {
			return c.PayloadType
		}
	}

	return PayloadType(0)
}

// parseREDFmtp parses RFC 2198's slash-separated payload type list. The
// copy-Opus profile requires at least one redundant encoding and a primary
// encoding, so a single payload type is not valid.
func parseREDFmtp(line string) ([]PayloadType, bool) {
	parts := strings.Split(line, "/")
	if len(parts) < 2 {
		return nil, false
	}

	payloadTypes := make([]PayloadType, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return nil, false
			}
		}

		payloadType, err := strconv.ParseUint(part, 10, 7)
		if err != nil {
			return nil, false
		}
		payloadTypes = append(payloadTypes, PayloadType(payloadType))
	}

	return payloadTypes, true
}

func parseCopyREDPrimaryPayloadType(line string) (PayloadType, bool) {
	payloadTypes, ok := parseREDFmtp(line)
	if !ok {
		return 0, false
	}

	primaryPayloadType := payloadTypes[0]
	for _, payloadType := range payloadTypes[1:] {
		if payloadType != primaryPayloadType {
			return 0, false
		}
	}

	return primaryPayloadType, true
}

// primaryPayloadTypeForRED returns the primary payload type associated with a
// supported RED codec and reports whether that primary is a negotiated Opus
// codec. Copy-Opus RED only supports repeated references to the same codec.
func primaryPayloadTypeForRED(needle RTPCodecParameters, haystack []RTPCodecParameters) (
	isRED bool, primaryPayloadType PayloadType, primaryExists bool,
) {
	if !strings.EqualFold(needle.MimeType, MimeTypeRED) {
		return false, 0, false
	}

	primaryPayloadType, ok := parseCopyREDPrimaryPayloadType(needle.SDPFmtpLine)
	if !ok {
		return true, 0, false
	}

	for _, codec := range haystack {
		if codec.PayloadType == primaryPayloadType && strings.EqualFold(codec.MimeType, MimeTypeOpus) {
			return true, primaryPayloadType, true
		}
	}

	return true, primaryPayloadType, false
}

// findREDPayloadType returns the RED payload type associated with an Opus
// payload type, if the codec list contains a valid copy-Opus RED pair.
func findREDPayloadType(opusPayloadType PayloadType, codecs []RTPCodecParameters) PayloadType {
	for _, codec := range codecs {
		isRED, primaryPayloadType, primaryExists := primaryPayloadTypeForRED(codec, codecs)
		if isRED && primaryExists && primaryPayloadType == opusPayloadType {
			return codec.PayloadType
		}
	}

	return 0
}

func opusREDCodecParameters(codecs []RTPCodecParameters) (RTPCodecParameters, PayloadType, bool) {
	for _, codec := range codecs {
		if !strings.EqualFold(codec.MimeType, MimeTypeOpus) {
			continue
		}
		if redPayloadType := findREDPayloadType(codec.PayloadType, codecs); redPayloadType != PayloadType(0) {
			return codec, redPayloadType, true
		}
	}

	return RTPCodecParameters{}, 0, false
}

// Given needle CodecParameters, returns if needle is RTX and
// if primary codec corresponding to that needle is in the haystack of codecs.
func primaryPayloadTypeForRTXExists(needle RTPCodecParameters, haystack []RTPCodecParameters) (
	isRTX bool, primaryExists bool,
) {
	if !strings.EqualFold(needle.MimeType, MimeTypeRTX) {
		return
	}

	isRTX = true
	parsed := fmtp.Parse(needle.MimeType, needle.ClockRate, needle.Channels, needle.SDPFmtpLine)
	aptPayload, ok := parsed.Parameter("apt")
	if !ok {
		return
	}

	primaryPayloadType, err := strconv.Atoi(aptPayload)
	if err != nil || primaryPayloadType < 0 || primaryPayloadType > 255 {
		return
	}

	for _, c := range haystack {
		if c.PayloadType == PayloadType(primaryPayloadType) {
			primaryExists = true

			return
		}
	}

	return
}

// Filter out RTX codecs that do not have a primary codec.
func filterUnattachedRTX(codecs []RTPCodecParameters) []RTPCodecParameters {
	for i := len(codecs) - 1; i >= 0; i-- {
		c := codecs[i]
		if isRTX, primaryExists := primaryPayloadTypeForRTXExists(c, codecs); isRTX && !primaryExists {
			// no primary for RTX, remove the RTX
			codecs = append(codecs[:i], codecs[i+1:]...)
		}
	}

	return codecs
}

// Filter out RED codecs that are malformed or do not have an associated Opus codec.
func filterUnattachedRED(codecs []RTPCodecParameters) []RTPCodecParameters {
	for i := len(codecs) - 1; i >= 0; i-- {
		if isRED, _, primaryExists := primaryPayloadTypeForRED(codecs[i], codecs); isRED && !primaryExists {
			codecs = append(codecs[:i], codecs[i+1:]...)
		}
	}

	return codecs
}

// For now, only FlexFEC is supported.
func findFECPayloadType(haystack []RTPCodecParameters) PayloadType {
	for _, c := range haystack {
		if strings.Contains(c.RTPCodecCapability.MimeType, MimeTypeFlexFEC) {
			return c.PayloadType
		}
	}

	return PayloadType(0)
}

func rtcpFeedbackIntersection(a, b []RTCPFeedback) (out []RTCPFeedback) {
	for _, aFeedback := range a {
		for _, bFeeback := range b {
			if aFeedback.Type == bFeeback.Type && aFeedback.Parameter == bFeeback.Parameter {
				out = append(out, aFeedback)

				break
			}
		}
	}

	return
}
