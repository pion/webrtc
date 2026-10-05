// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build opusred && !js

package opusredmedia

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	oggHeaderBytes  = 27
	oggContinuation = 1
	oggBeginning    = 2
	oggEnd          = 4
	oggSerial       = uint32(0x52454401)
)

var (
	errOgg            = errors.New("invalid fixture Ogg stream")
	errMetadata       = errors.New("invalid stereo 48 kHz Opus metadata")
	errPacketDuration = errors.New("invalid Opus packet duration")
	errEmptyCorpus    = errors.New("opus corpus is empty")
	errLargeOggPacket = errors.New("opus packet is too large for a single qualification Ogg page")
)

type oggPage struct {
	flags             byte
	granule           uint64
	serial, sequence  uint32
	segments, payload []byte
}

type oggStream struct {
	packets          [][]byte
	pending          []byte
	serial, sequence uint32
	started, ended   bool
}

// extractOpusPackets follows Ogg lacing, including packet continuations and
// multiple packets per page. Page payloads are never mistaken for Opus packets.
func extractOpusPackets(data []byte) (*Fixture, error) {
	var stream oggStream
	for len(data) > 0 {
		page, remaining, err := readOggPage(data)
		if err != nil {
			return nil, err
		}
		if err = stream.consume(page); err != nil {
			return nil, err
		}
		data = remaining
	}

	return stream.fixture()
}

func readOggPage(data []byte) (oggPage, []byte, error) {
	var result oggPage
	if len(data) < oggHeaderBytes || string(data[:4]) != "OggS" || data[4] != 0 {
		return result, nil, errOgg
	}
	flags := data[5]
	if flags & ^byte(oggContinuation|oggBeginning|oggEnd) != 0 {
		return result, nil, errOgg
	}
	headerLength := oggHeaderBytes + int(data[26])
	if len(data) < headerLength {
		return result, nil, fmt.Errorf("%w: truncated lacing table", errOgg)
	}
	payloadLength := 0
	for _, length := range data[oggHeaderBytes:headerLength] {
		payloadLength += int(length)
	}
	pageLength := headerLength + payloadLength
	if len(data) < pageLength {
		return result, nil, fmt.Errorf("%w: truncated page payload", errOgg)
	}
	page := data[:pageLength]
	if binary.LittleEndian.Uint32(page[22:26]) != oggChecksum(page) {
		return result, nil, fmt.Errorf("%w: checksum mismatch", errOgg)
	}
	result = oggPage{
		flags:    flags,
		granule:  binary.LittleEndian.Uint64(page[6:14]),
		serial:   binary.LittleEndian.Uint32(page[14:18]),
		sequence: binary.LittleEndian.Uint32(page[18:22]),
		segments: page[oggHeaderBytes:headerLength],
		payload:  page[headerLength:],
	}

	return result, data[pageLength:], nil
}

func (stream *oggStream) consume(page oggPage) error {
	if err := stream.validatePosition(page); err != nil {
		return err
	}
	if (page.flags&oggContinuation != 0) != (len(stream.pending) != 0) {
		return fmt.Errorf("%w: inconsistent packet continuation", errOgg)
	}
	stream.appendSegments(page)
	if page.sequence == 0 {
		if len(stream.packets) != 1 || len(stream.pending) != 0 {
			return fmt.Errorf("%w: OpusHead must occupy its own beginning page", errOgg)
		}
	}
	stream.ended = page.flags&oggEnd != 0
	if stream.ended && len(stream.pending) != 0 {
		return fmt.Errorf("%w: unfinished final packet", errOgg)
	}
	stream.sequence = page.sequence + 1

	return nil
}

func (stream *oggStream) validatePosition(page oggPage) error {
	if stream.ended {
		return fmt.Errorf("%w: data follows final page", errOgg)
	}
	if !stream.started {
		if page.flags != oggBeginning || page.sequence != 0 || page.granule != 0 {
			return fmt.Errorf("%w: invalid beginning page", errOgg)
		}
		stream.serial = page.serial
		stream.started = true
	} else if page.flags&oggBeginning != 0 || page.serial != stream.serial || page.sequence != stream.sequence {
		return fmt.Errorf("%w: unexpected stream or page sequence", errOgg)
	}

	return nil
}

func (stream *oggStream) appendSegments(page oggPage) {
	offset := 0
	for _, length := range page.segments {
		next := offset + int(length)
		stream.pending = append(stream.pending, page.payload[offset:next]...)
		offset = next
		if length < 255 {
			stream.packets = append(stream.packets, stream.pending)
			stream.pending = nil
		}
	}
}

func (stream *oggStream) fixture() (*Fixture, error) {
	if !stream.ended || len(stream.pending) != 0 || len(stream.packets) < 3 {
		return nil, fmt.Errorf("%w: incomplete stream", errOgg)
	}
	if err := validateOpusHead(stream.packets[0]); err != nil {
		return nil, err
	}
	if err := validateOpusTags(stream.packets[1]); err != nil {
		return nil, err
	}
	for _, packet := range stream.packets[2:] {
		if len(packet) == 0 || bytes.HasPrefix(packet, []byte("OpusHead")) || bytes.HasPrefix(packet, []byte("OpusTags")) {
			return nil, fmt.Errorf("%w: empty or unexpected metadata packet", errOgg)
		}
	}

	return &Fixture{OpusHead: stream.packets[0], OpusTags: stream.packets[1], Packets: stream.packets[2:]}, nil
}

func validateOpusHead(head []byte) error {
	if len(head) != 19 || string(head[:8]) != "OpusHead" || head[8] != 1 || head[9] != Channels ||
		binary.LittleEndian.Uint32(head[12:16]) != SampleRate || head[18] != 0 {
		return errMetadata
	}

	return nil
}

func validateOpusTags(tags []byte) error {
	if len(tags) < 16 || string(tags[:8]) != "OpusTags" {
		return errMetadata
	}
	vendorLength := binary.LittleEndian.Uint32(tags[8:12])
	if uint64(vendorLength)+16 > uint64(len(tags)) {
		return errMetadata
	}
	offset := 12 + int(vendorLength)
	commentCount := binary.LittleEndian.Uint32(tags[offset : offset+4])
	offset += 4
	if uint64(commentCount) > uint64((len(tags)-offset)/4) { //nolint:gosec // Offset was validated within tags above.
		return errMetadata
	}
	for range commentCount {
		if len(tags)-offset < 4 {
			return errMetadata
		}
		length := binary.LittleEndian.Uint32(tags[offset : offset+4])
		offset += 4
		if uint64(length) > uint64(len(tags)-offset) { //nolint:gosec // Checked four bytes remain before advancing offset.
			return errMetadata
		}
		offset += int(length)
	}

	return nil
}

func marshalCorpus(fixture *Fixture, payloads [][]byte) ([]byte, error) {
	if len(payloads) == 0 {
		return nil, errEmptyCorpus
	}
	var output bytes.Buffer
	if err := writeCorpusMetadata(&output, fixture); err != nil {
		return nil, err
	}
	var granule uint64
	for index, packet := range payloads {
		samples, err := opusPacketSamples(packet)
		if err != nil || samples != samplesPerPacket {
			return nil, fmt.Errorf("corpus packet %d is not 20 ms Opus: %w", index+1, errors.Join(err, errPacketDuration))
		}
		granule += samplesPerPacket
		flags := byte(0)
		if index == len(payloads)-1 {
			flags = oggEnd
		}
		page, err := singlePacketPage(packet, flags, granule, uint32(index+2)) //nolint:gosec // Qualification corpora are small.
		if err != nil {
			return nil, err
		}
		_, _ = output.Write(page)
	}

	return output.Bytes(), nil
}

func writeCorpusMetadata(output *bytes.Buffer, fixture *Fixture) error {
	if err := validateOpusHead(fixture.OpusHead); err != nil {
		return err
	}
	if err := validateOpusTags(fixture.OpusTags); err != nil {
		return err
	}
	for index, metadata := range [][]byte{fixture.OpusHead, fixture.OpusTags} {
		flags := byte(0)
		if index == 0 {
			flags = oggBeginning
		}
		page, err := singlePacketPage(metadata, flags, 0, uint32(index)) //nolint:gosec // Two metadata packets only.
		if err != nil {
			return err
		}
		_, _ = output.Write(page)
	}

	return nil
}

func singlePacketPage(packet []byte, flags byte, granule uint64, sequence uint32) ([]byte, error) {
	// A terminal zero lace is required when the packet length is a multiple
	// of 255. The qualification fixtures fit comfortably in one Ogg page.
	segmentCount := len(packet)/255 + 1
	if segmentCount > 255 {
		return nil, errLargeOggPacket
	}
	segments := make([]byte, segmentCount)
	for index := range segmentCount - 1 {
		segments[index] = 255
	}
	segments[segmentCount-1] = byte(len(packet) % 255) //nolint:gosec // Remainder is at most 254.

	return makeOggPage(flags, granule, oggSerial, sequence, segments, packet), nil
}

func makeOggPage(flags byte, granule uint64, serial, sequence uint32, segments, payload []byte) []byte {
	page := make([]byte, oggHeaderBytes+len(segments)+len(payload))
	copy(page, "OggS")
	page[5] = flags
	binary.LittleEndian.PutUint64(page[6:14], granule)
	binary.LittleEndian.PutUint32(page[14:18], serial)
	binary.LittleEndian.PutUint32(page[18:22], sequence)
	page[26] = byte(len(segments)) //nolint:gosec // Callers bound lacing tables to 255 entries.
	copy(page[oggHeaderBytes:], segments)
	copy(page[oggHeaderBytes+len(segments):], payload)
	binary.LittleEndian.PutUint32(page[22:26], oggChecksum(page))

	return page
}

func oggChecksum(page []byte) uint32 {
	var checksum uint32
	for index, value := range page {
		if index >= 22 && index < 26 {
			value = 0
		}
		checksum ^= uint32(value) << 24
		for range 8 {
			if checksum&0x80000000 != 0 {
				checksum = checksum<<1 ^ 0x04c11db7
			} else {
				checksum <<= 1
			}
		}
	}

	return checksum
}

// opusPacketSamples derives duration from the Opus TOC, independent of Ogg
// page granules. This is not a substitute for the real FFmpeg decode check.
func opusPacketSamples(packet []byte) (int, error) {
	if len(packet) == 0 {
		return 0, errPacketDuration
	}
	configuration := packet[0] >> 3
	var frameSamples int
	switch {
	case configuration >= 16:
		frameSamples = 120 << (configuration & 3)
	case configuration >= 12:
		frameSamples = 480 << (configuration & 1)
	default:
		frameSamples = []int{480, 960, 1920, 2880}[configuration&3]
	}
	frames := 1
	switch packet[0] & 3 {
	case 1, 2:
		frames = 2
	case 3:
		if len(packet) < 2 {
			return 0, errPacketDuration
		}
		frames = int(packet[1] & 0x3f)
	}
	samples := frameSamples * frames
	if frames == 0 || samples > 5760 {
		return 0, errPacketDuration
	}

	return samples, nil
}
