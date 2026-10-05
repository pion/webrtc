// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build opusred && !js

package opusredmedia

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func testMetadata() *Fixture {
	head := make([]byte, 19)
	copy(head, "OpusHead")
	head[8], head[9] = 1, Channels
	binary.LittleEndian.PutUint16(head[10:12], 312)
	binary.LittleEndian.PutUint32(head[12:16], SampleRate)
	tags := make([]byte, 16)
	copy(tags, "OpusTags")

	return &Fixture{OpusHead: head, OpusTags: tags}
}

func testHeaders(t *testing.T, fixture *Fixture) []byte {
	t.Helper()
	head, err := singlePacketPage(fixture.OpusHead, oggBeginning, 0, 0)
	if err != nil {
		require.NoError(t, err)
	}
	tags, err := singlePacketPage(fixture.OpusTags, 0, 0, 1)
	if err != nil {
		require.NoError(t, err)
	}

	return append(head, tags...)
}

func TestExtractOpusPacketsLacing(t *testing.T) {
	for _, packetSize := range []int{3, 255, 256, 510, 511} {
		t.Run(stringName(packetSize), func(t *testing.T) {
			packet := bytes.Repeat([]byte{0x55}, packetSize)
			packet[0] = 0xfc // CELT stereo, one 20 ms frame.
			fixture := testMetadata()
			data, err := marshalCorpus(fixture, [][]byte{packet})
			if err != nil {
				require.NoError(t, err)
			}
			parsed, err := extractOpusPackets(data)
			if err != nil {
				require.NoError(t, err)
			}
			if len(parsed.Packets) != 1 || !bytes.Equal(parsed.Packets[0], packet) {
				require.FailNow(t, "Ogg laces changed the Opus packet boundary")
			}
			if !bytes.Equal(parsed.OpusHead, fixture.OpusHead) || !bytes.Equal(parsed.OpusTags, fixture.OpusTags) {
				require.FailNow(t, "codec metadata, including pre-skip, was not preserved")
			}
			// The returned packet is owned independently of the encoded file.
			for index := range data {
				data[index] = 0
			}
			if !bytes.Equal(parsed.Packets[0], packet) {
				require.FailNow(t, "returned packet aliases the Ogg source buffer")
			}
		})
	}
}

func TestExtractOpusPacketsMultiplePacketsPerPage(t *testing.T) {
	first := []byte{0xfc, 0x01, 0x02}
	second := bytes.Repeat([]byte{0x44}, 256)
	second[0] = 0xfc
	third := []byte{0xfc, 0x03}
	payload := append(append(append([]byte{}, first...), second...), third...)
	page := makeOggPage(oggEnd, 3*samplesPerPacket, oggSerial, 2, []byte{3, 255, 1, 2}, payload)
	parsed, err := extractOpusPackets(append(testHeaders(t, testMetadata()), page...))
	if err != nil {
		require.NoError(t, err)
	}
	if !reflect.DeepEqual(parsed.Packets, [][]byte{first, second, third}) {
		require.FailNowf(t, "qualification assertion failed", "expected three individual Opus packets, got lengths %v", packetLengths(parsed.Packets))
	}
}

func TestExtractOpusPacketsContinuation(t *testing.T) {
	for _, size := range []int{255, 300, 510, 560} {
		t.Run(stringName(size), func(t *testing.T) {
			packet := bytes.Repeat([]byte{0x42}, size)
			packet[0] = 0xfc
			firstPage := makeOggPage(0, ^uint64(0), oggSerial, 2, []byte{255}, packet[:255])
			remaining := packet[255:]
			segments := []byte{}
			for count := len(remaining); count >= 255; count -= 255 {
				segments = append(segments, 255)
			}
			segments = append(segments, byte(len(remaining)%255))
			finalPage := makeOggPage(oggContinuation|oggEnd, samplesPerPacket, oggSerial, 3, segments, remaining)
			data := append(testHeaders(t, testMetadata()), firstPage...)
			data = append(data, finalPage...)
			parsed, err := extractOpusPackets(data)
			if err != nil {
				require.NoError(t, err)
			}
			if len(parsed.Packets) != 1 || !bytes.Equal(parsed.Packets[0], packet) {
				require.FailNow(t, "continued packet was split or corrupted")
			}
		})
	}
}

func TestExtractOpusPacketsRejectsCorruption(t *testing.T) {
	valid, err := marshalCorpus(testMetadata(), [][]byte{{0xfc, 1, 2}})
	if err != nil {
		require.NoError(t, err)
	}
	t.Run("EveryTruncatedPrefix", func(t *testing.T) {
		for size := range len(valid) {
			if _, parseErr := extractOpusPackets(valid[:size]); parseErr == nil {
				require.FailNowf(t, "qualification assertion failed", "accepted a truncated stream of %d bytes", size)
			}
		}
	})
	t.Run("ChecksumMismatch", func(t *testing.T) {
		corrupt := append([]byte{}, valid...)
		corrupt[len(corrupt)-1] ^= 1
		if _, parseErr := extractOpusPackets(corrupt); parseErr == nil {
			require.FailNow(t, "accepted corrupt Opus bytes with an invalid Ogg checksum")
		}
	})
	t.Run("TrailingData", func(t *testing.T) {
		if _, parseErr := extractOpusPackets(append(valid, 0)); parseErr == nil {
			require.FailNow(t, "accepted trailing bytes after the final page")
		}
	})
	for name, page := range map[string][]byte{
		"UnexpectedContinuation": makeOggPage(oggContinuation|oggEnd, 960, oggSerial, 2, []byte{3}, []byte{0xfc, 1, 2}),
		"WrongSerial":            makeOggPage(oggEnd, 960, oggSerial+1, 2, []byte{3}, []byte{0xfc, 1, 2}),
		"MissingPage":            makeOggPage(oggEnd, 960, oggSerial, 3, []byte{3}, []byte{0xfc, 1, 2}),
		"UnfinishedFinalPacket":  makeOggPage(oggEnd, 960, oggSerial, 2, []byte{255}, bytes.Repeat([]byte{1}, 255)),
		"EmptyAudioPacket":       makeOggPage(oggEnd, 960, oggSerial, 2, []byte{0}, nil),
	} {
		t.Run(name, func(t *testing.T) {
			if _, parseErr := extractOpusPackets(append(testHeaders(t, testMetadata()), page...)); parseErr == nil {
				require.FailNow(t, "accepted invalid stream structure")
			}
		})
	}
	t.Run("MissingContinuationFlag", func(t *testing.T) {
		first := makeOggPage(0, ^uint64(0), oggSerial, 2, []byte{255}, bytes.Repeat([]byte{1}, 255))
		last := makeOggPage(oggEnd, 960, oggSerial, 3, []byte{2}, []byte{2, 3})
		data := append(append(testHeaders(t, testMetadata()), first...), last...)
		if _, parseErr := extractOpusPackets(data); parseErr == nil {
			require.FailNow(t, "accepted a continuation without its flag")
		}
	})
}

func TestExtractOpusPacketsRejectsInvalidMetadata(t *testing.T) {
	for name, mutate := range map[string]func(*Fixture){
		"Mono":               func(f *Fixture) { f.OpusHead[9] = 1 },
		"WrongRate":          func(f *Fixture) { binary.LittleEndian.PutUint32(f.OpusHead[12:16], 16000) },
		"UnsupportedMapping": func(f *Fixture) { f.OpusHead[18] = 1 },
		"UnsupportedVersion": func(f *Fixture) { f.OpusHead[8] = 2 },
		"WrongHeadSignature": func(f *Fixture) { f.OpusHead[0] = 'X' },
		"TruncatedHead":      func(f *Fixture) { f.OpusHead = f.OpusHead[:18] },
		"WrongTagsSignature": func(f *Fixture) { f.OpusTags[0] = 'X' },
		"TruncatedVendor":    func(f *Fixture) { binary.LittleEndian.PutUint32(f.OpusTags[8:12], ^uint32(0)) },
		"TruncatedComments":  func(f *Fixture) { binary.LittleEndian.PutUint32(f.OpusTags[12:16], 1) },
	} {
		t.Run(name, func(t *testing.T) {
			fixture := testMetadata()
			mutate(fixture)
			page := makeOggPage(oggEnd, samplesPerPacket, oggSerial, 2, []byte{3}, []byte{0xfc, 1, 2})
			_, parseErr := extractOpusPackets(append(testHeaders(t, fixture), page...))
			if !errors.Is(parseErr, errMetadata) {
				require.FailNowf(t, "qualification assertion failed", "expected invalid stereo metadata, got %v", parseErr)
			}
		})
	}
}

func TestOpusPacketSamples(t *testing.T) {
	cases := []struct {
		name    string
		packet  []byte
		samples int
	}{
		{"CELT2_5ms", []byte{0x80}, 120},
		{"CELT10ms", []byte{0x90}, 480},
		{"CELT20msStereo", []byte{0xfc, 1, 2}, 960},
		{"Hybrid10ms", []byte{0x60}, 480},
		{"Hybrid20ms", []byte{0x68}, 960},
		{"SILK60ms", []byte{0x18}, 2880},
		{"TwoSILK60ms", []byte{0x1a}, 5760},
		{"FortyEightShortFrames", []byte{0x83, 48}, 5760},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := opusPacketSamples(test.packet)
			if err != nil || got != test.samples {
				require.FailNowf(t, "qualification assertion failed", "duration = %d, %v; want %d", got, err, test.samples)
			}
		})
	}
	for _, invalid := range [][]byte{nil, {0x83}, {0x83, 0}, {0x83, 49}} {
		if _, err := opusPacketSamples(invalid); err == nil {
			require.FailNowf(t, "qualification assertion failed", "accepted invalid duration TOC: %x", invalid)
		}
	}
}

func TestGenerateReportsMissingPrerequisite(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Generate(context.Background())
	var prerequisite *ErrPrerequisite
	if !errors.As(err, &prerequisite) || prerequisite.Tool != "ffmpeg" {
		require.FailNowf(t, "qualification assertion failed", "expected typed missing-ffmpeg prerequisite, got %v", err)
	}
}

func packetLengths(packets [][]byte) []int {
	lengths := make([]int, len(packets))
	for index, packet := range packets {
		lengths[index] = len(packet)
	}

	return lengths
}

func stringName(value int) string {
	// Keep these boundary subtests individually selectable without importing
	// any runtime audio tooling.
	return fmt.Sprintf("Bytes%d", value)
}

func TestLibopusEncoderDetection(t *testing.T) {
	for _, available := range []string{
		" A..... libopus             libopus Opus (codec opus)",
		"Encoders:\n A....D libopus libopus Opus\n A..... pcm_s16le PCM",
	} {
		if !hasLibopusEncoder([]byte(available)) {
			require.FailNow(t, "did not detect the available libopus audio encoder")
		}
	}
	for _, missing := range []string{"", " A..... opus Opus", "libopus decoder", " V..... libopus video"} {
		if hasLibopusEncoder([]byte(missing)) {
			require.FailNow(t, "mistook another codec or decoder for the libopus encoder")
		}
	}
}
