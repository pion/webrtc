//go:build opusred && !js

// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package opusredtest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/sdp/v3"
)

// Mapping is immutable negotiation metadata for one audio trial.
type Mapping struct {
	OpusPT    uint8  `json:"opusPT"`
	REDPT     uint8  `json:"redPT"`
	ClockRate uint32 `json:"clockRate"`
	Channels  uint16 `json:"channels"`
	OpusFMTP  string `json:"opusFMTP"`
	REDFMTP   string `json:"redFMTP"`
	HasRED    bool   `json:"hasRED"`
}

type codec struct {
	name     string
	clock    uint32
	channels uint16
}
type selectedAudio struct {
	mapping Mapping
	mid     string
	formats map[uint8]bool
	codecs  map[uint8]codec
}

func parsePT(value string) (uint8, error) {
	number, err := strconv.ParseUint(value, 10, 7)
	if err != nil {
		return 0, fmt.Errorf("invalid RTP payload type %q", value)
	}
	return uint8(number), nil
}

func validateMapping(mapping Mapping) error {
	if mapping.OpusPT > 127 || mapping.ClockRate != 48000 || mapping.Channels != 2 {
		return fmt.Errorf("Opus requires a 7-bit PT and rtpmap opus/48000/2")
	}
	if !mapping.HasRED {
		return nil
	}
	if mapping.REDPT == 0 || mapping.REDPT > 127 || mapping.REDPT == mapping.OpusPT {
		return fmt.Errorf("RED requires a distinct nonzero 7-bit PT")
	}
	references := strings.Split(mapping.REDFMTP, "/")
	for _, reference := range references {
		pt, err := parsePT(strings.TrimSpace(reference))
		if err != nil || pt != mapping.OpusPT {
			return fmt.Errorf("RED fmtp must reference only negotiated Opus PT %d", mapping.OpusPT)
		}
	}
	return nil
}

func selectAudio(raw string) (selectedAudio, error) {
	var description sdp.SessionDescription
	if err := description.Unmarshal([]byte(raw)); err != nil {
		return selectedAudio{}, fmt.Errorf("parse SDP: %w", err)
	}
	var media *sdp.MediaDescription
	for _, candidate := range description.MediaDescriptions {
		if candidate.MediaName.Media != "audio" || candidate.MediaName.Port.Value == 0 {
			continue
		}
		if media != nil {
			return selectedAudio{}, fmt.Errorf("trial requires one active audio m-line")
		}
		media = candidate
	}
	if media == nil {
		return selectedAudio{}, fmt.Errorf("SDP has no active audio m-line")
	}
	formats := make(map[uint8]bool)
	order := make([]uint8, 0, len(media.MediaName.Formats))
	for _, value := range media.MediaName.Formats {
		pt, err := parsePT(value)
		if err != nil {
			return selectedAudio{}, err
		}
		if formats[pt] {
			return selectedAudio{}, fmt.Errorf("duplicate audio payload type %d", pt)
		}
		formats[pt] = true
		order = append(order, pt)
	}
	codecs := make(map[uint8]codec)
	parameters := make(map[uint8]string)
	for _, attribute := range media.Attributes {
		if attribute.Key != "rtpmap" && attribute.Key != "fmtp" {
			continue
		}
		fields := strings.Fields(attribute.Value)
		if len(fields) < 2 {
			return selectedAudio{}, fmt.Errorf("malformed %s", attribute.Key)
		}
		pt, err := parsePT(fields[0])
		if err != nil {
			return selectedAudio{}, err
		}
		if !formats[pt] {
			return selectedAudio{}, fmt.Errorf("%s references unoffered PT %d", attribute.Key, pt)
		}
		if attribute.Key == "fmtp" {
			if _, exists := parameters[pt]; exists {
				return selectedAudio{}, fmt.Errorf("duplicate fmtp for PT %d", pt)
			}
			parameters[pt] = strings.TrimSpace(strings.TrimPrefix(attribute.Value, fields[0]))
			continue
		}
		if len(fields) != 2 {
			return selectedAudio{}, fmt.Errorf("malformed rtpmap for PT %d", pt)
		}
		if _, exists := codecs[pt]; exists {
			return selectedAudio{}, fmt.Errorf("duplicate rtpmap for PT %d", pt)
		}
		parts := strings.Split(fields[1], "/")
		if len(parts) < 2 || len(parts) > 3 {
			return selectedAudio{}, fmt.Errorf("malformed rtpmap codec")
		}
		rate, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil || rate == 0 {
			return selectedAudio{}, fmt.Errorf("invalid rtpmap clock")
		}
		channels := uint64(1)
		if len(parts) == 3 {
			channels, err = strconv.ParseUint(parts[2], 10, 16)
			if err != nil || channels == 0 {
				return selectedAudio{}, fmt.Errorf("invalid rtpmap channels")
			}
		}
		codecs[pt] = codec{name: strings.ToLower(parts[0]), clock: uint32(rate), channels: uint16(channels)}
	}
	var mapping Mapping
	foundOpus := false
	for _, pt := range order {
		candidate := codecs[pt]
		if candidate.name == "opus" {
			if foundOpus {
				return selectedAudio{}, fmt.Errorf("trial requires one unambiguous Opus mapping")
			}
			mapping = Mapping{OpusPT: pt, ClockRate: candidate.clock, Channels: candidate.channels, OpusFMTP: parameters[pt]}
			foundOpus = true
		}
	}
	if !foundOpus {
		return selectedAudio{}, fmt.Errorf("audio m-line lacks Opus")
	}
	for _, pt := range order {
		candidate := codecs[pt]
		if candidate.name != "red" {
			continue
		}
		if mapping.HasRED {
			return selectedAudio{}, fmt.Errorf("trial requires one RED mapping")
		}
		if candidate.clock != mapping.ClockRate || candidate.channels != mapping.Channels {
			return selectedAudio{}, fmt.Errorf("RED and Opus clock/channels differ")
		}
		mapping.HasRED, mapping.REDPT, mapping.REDFMTP = true, pt, parameters[pt]
	}
	if err := validateMapping(mapping); err != nil {
		return selectedAudio{}, err
	}
	mid, _ := media.Attribute("mid")
	return selectedAudio{mapping: mapping, mid: mid, formats: formats, codecs: codecs}, nil
}

// SelectOffer selects one unambiguous active audio mapping, including plain Opus fallback.
// Trials with multiple Opus profiles are rejected rather than choosing a profile implicitly.
func SelectOffer(raw string) (Mapping, error) {
	selected, err := selectAudio(raw)
	return selected.mapping, err
}

// ValidateAnswer rejects introduced or remapped codecs and returns accepted audio metadata.
func ValidateAnswer(offer, answer string) (Mapping, error) {
	offered, err := selectAudio(offer)
	if err != nil {
		return Mapping{}, fmt.Errorf("offer: %w", err)
	}
	accepted, err := selectAudio(answer)
	if err != nil {
		return Mapping{}, fmt.Errorf("answer: %w", err)
	}
	if offered.mid != accepted.mid {
		return Mapping{}, fmt.Errorf("answer changed audio MID")
	}
	for pt := range accepted.formats {
		if !offered.formats[pt] {
			return Mapping{}, fmt.Errorf("answer introduced unoffered audio PT %d", pt)
		}
		if offered.codecs[pt] != accepted.codecs[pt] {
			return Mapping{}, fmt.Errorf("answer changed offered codec mapping for PT %d", pt)
		}
	}
	left, right := offered.mapping, accepted.mapping
	if left.OpusPT != right.OpusPT || left.ClockRate != right.ClockRate || left.Channels != right.Channels {
		return Mapping{}, fmt.Errorf("answer changed offered Opus mapping")
	}
	if right.HasRED && (!left.HasRED || right.REDPT != left.REDPT) {
		return Mapping{}, fmt.Errorf("answer introduced or remapped RED")
	}
	return right, nil
}
