// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build opusred && !js

// Package opusredmedia prepares real Opus audio and independent decode evidence
// for the opt-in Opus RED qualification tests.
package opusredmedia

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	// PacketCount is the number of 20 ms packets in each Test 01 trial.
	PacketCount           = 300
	SampleRate            = 48000
	Channels              = 2
	samplesPerPacket      = 960
	maxFixturePacketBytes = 350
	commandTimeout        = 30 * time.Second
)

var (
	errFixture        = errors.New("invalid Opus fixture")
	errArtifactPrefix = errors.New("audio artifact prefix must contain only letters, digits, hyphens or underscores")
	errNilFixture     = errors.New("audio fixture is nil")
	errPCM            = errors.New("invalid decoded stereo PCM")
	errMissingLibopus = errors.New("FFmpeg was built without the libopus encoder")
)

// Fixture contains owned Opus packets and their original Ogg codec metadata.
// SHA256 covers OpusHead and the length-prefixed selected packets, excluding
// the random Ogg stream serial and tool-generated comments.
type Fixture struct {
	Packets       [][]byte `json:"packets"`
	SHA256        string   `json:"sha256"`
	FFmpegVersion string   `json:"ffmpegVersion"`
	OpusHead      []byte   `json:"opusHead"`
	OpusTags      []byte   `json:"opusTags"`
}

// ErrPrerequisite identifies unavailable external tooling. A runner should
// report this as BLOCKED rather than silently skipping audio qualification.
type ErrPrerequisite struct { //nolint:errname // Shared qualification API deliberately identifies prerequisites as ErrPrerequisite.
	Tool string
	Err  error
}

func (err *ErrPrerequisite) Error() string {
	return fmt.Sprintf("required audio tool %s is unavailable: %v", err.Tool, err.Err)
}

func (err *ErrPrerequisite) Unwrap() error { return err.Err }

// AudioEvidence describes decoded 48 kHz stereo signed 16-bit PCM and its
// playable Ogg/WAV artifacts. RMS uses full scale as 1.0 across both channels.
type AudioEvidence struct {
	PCMSHA256       string  `json:"pcmSHA256"` //nolint:tagliatelle // Shared evidence schema preserves the SHA256 acronym.
	PCMBytes        int     `json:"pcmBytes"`
	DurationSeconds float64 `json:"durationSeconds"`
	RMS             float64 `json:"rms"`
	OggPath         string  `json:"oggPath"`
	WAVPath         string  `json:"wavPath"`
	PCMPath         string  `json:"pcmPath"`
}

// Generate encodes a deterministic, non-silent stereo signal with libopus.
// Call once per qualification run and reuse the returned fixture in both trials.
func Generate(ctx context.Context) (*Fixture, error) {
	ffmpeg, err := findFFmpeg()
	if err != nil {
		return nil, err
	}
	version, err := runFFmpeg(ctx, ffmpeg, "-version")
	if err != nil {
		return nil, err
	}
	if err = requireLibopusEncoder(ctx, ffmpeg); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "opus-red-fixture-")
	if err != nil {
		return nil, fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck // Temporary encoder artifacts are disposable.
	path := filepath.Join(dir, "fixture.ogg")
	// Different continuously varying signals in each channel make stereo and
	// non-silent decoding observable without microphone access or media downloads.
	signal := "aevalsrc=0.16*sin(2*PI*(440+80*sin(2*PI*0.7*t))*t)+0.04*sin(2*PI*1200*t)|" +
		"0.16*sin(2*PI*(660+60*sin(2*PI*0.5*t))*t)+0.04*sin(2*PI*1700*t):s=48000:d=6.2"
	_, err = runFFmpeg(ctx, ffmpeg,
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-f", "lavfi", "-i", signal, "-map_metadata", "-1",
		"-fflags", "+bitexact", "-flags:a", "+bitexact",
		"-c:a", "libopus", "-application", "audio", "-b:a", "48000",
		"-vbr", "on", "-frame_duration", "20", "-fec", "0", "-packet_loss", "0",
		"-ar", "48000", "-ac", "2", "-f", "ogg", path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path) //nolint:gosec // Path is inside this function's newly created temporary directory.
	if err != nil {
		return nil, fmt.Errorf("read encoded fixture: %w", err)
	}
	fixture, err := extractOpusPackets(data)
	if err != nil {
		return nil, fmt.Errorf("extract encoded fixture: %w", err)
	}
	if err = selectFixturePackets(fixture); err != nil {
		return nil, err
	}
	hash := sha256.New()
	_, _ = hash.Write(fixture.OpusHead)
	for _, packet := range fixture.Packets {
		var length [4]byte
		binary.LittleEndian.PutUint32(length[:], uint32(len(packet))) //nolint:gosec // Packet size is bounded above.
		_, _ = hash.Write(length[:])
		_, _ = hash.Write(packet)
	}
	fixture.SHA256 = hex.EncodeToString(hash.Sum(nil))
	fixture.FFmpegVersion = strings.SplitN(strings.TrimSpace(string(version)), "\n", 2)[0]

	return fixture, nil
}

func selectFixturePackets(fixture *Fixture) error {
	if len(fixture.Packets) < PacketCount {
		return fmt.Errorf("%w: fixture has %d Opus packets, need %d", errFixture, len(fixture.Packets), PacketCount)
	}
	fixture.Packets = fixture.Packets[:PacketCount:PacketCount]
	for index, packet := range fixture.Packets {
		samples, packetErr := opusPacketSamples(packet)
		if packetErr != nil || samples != samplesPerPacket {
			return fmt.Errorf("fixture packet %d is not 20 ms Opus: samples=%d: %w", index+1, samples,
				errors.Join(packetErr, errPacketDuration))
		}
		if len(packet) > maxFixturePacketBytes {
			return fmt.Errorf("%w: fixture packet %d has %d bytes, maximum %d for two-copy RED", errFixture, index+1,
				len(packet), maxFixturePacketBytes)
		}
	}

	return nil
}

// WriteCorpus wraps a complete ordered corpus using the fixture's original
// OpusHead (including pre-skip), decodes it, and records measurable audio evidence.
// Reference and received corpora deliberately use this same container path.
// This API does not fill missing packets or model concealment for lossy corpora.
func WriteCorpus(ctx context.Context, dir, prefix string, fixture *Fixture, payloads [][]byte) (AudioEvidence, error) {
	var evidence AudioEvidence
	ffmpeg, err := findFFmpeg()
	if err != nil {
		return evidence, err
	}
	if !validPrefix(prefix) {
		return evidence, errArtifactPrefix
	}
	if fixture == nil {
		return evidence, errNilFixture
	}
	ogg, err := marshalCorpus(fixture, payloads)
	if err != nil {
		return evidence, err
	}
	if err = os.MkdirAll(dir, 0o750); err != nil {
		return evidence, fmt.Errorf("create audio artifact directory: %w", err)
	}
	evidence.OggPath = filepath.Join(dir, prefix+".ogg")
	evidence.WAVPath = filepath.Join(dir, prefix+".wav")
	evidence.PCMPath = filepath.Join(dir, prefix+".pcm")
	if err = os.WriteFile(evidence.OggPath, ogg, 0o600); err != nil {
		return evidence, fmt.Errorf("write Opus corpus: %w", err)
	}
	_, err = runFFmpeg(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-i", evidence.OggPath,
		"-map", "0:a:0", "-ar", "48000", "-ac", "2", "-c:a", "pcm_s16le", "-f", "s16le", evidence.PCMPath,
		"-map", "0:a:0", "-ar", "48000", "-ac", "2", "-c:a", "pcm_s16le", "-f", "wav", evidence.WAVPath)
	if err != nil {
		return evidence, err
	}
	pcm, err := os.ReadFile(evidence.PCMPath)
	if err != nil {
		return evidence, fmt.Errorf("read decoded PCM: %w", err)
	}
	if err = inspectPCM(pcm, &evidence); err != nil {
		return evidence, err
	}

	return evidence, nil
}

func inspectPCM(pcm []byte, evidence *AudioEvidence) error {
	if len(pcm) == 0 || len(pcm)%(Channels*2) != 0 {
		return fmt.Errorf("%w: decoded sample length %d", errPCM, len(pcm))
	}
	var sumSquares float64
	for index := 0; index < len(pcm); index += 2 {
		sample := float64(int16(binary.LittleEndian.Uint16(pcm[index:index+2]))) / 32768 //nolint:gosec // Signed PCM reinterpretation.
		sumSquares += sample * sample
	}
	digest := sha256.Sum256(pcm)
	evidence.PCMSHA256 = hex.EncodeToString(digest[:])
	evidence.PCMBytes = len(pcm)
	evidence.DurationSeconds = float64(len(pcm)) / (SampleRate * Channels * 2)
	evidence.RMS = math.Sqrt(sumSquares / float64(len(pcm)/2))

	return nil
}

func findFFmpeg() (string, error) {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", &ErrPrerequisite{Tool: "ffmpeg", Err: err}
	}

	return path, nil
}

func requireLibopusEncoder(ctx context.Context, path string) error {
	output, err := runFFmpeg(ctx, path, "-hide_banner", "-encoders")
	if err != nil {
		return err
	}
	if !hasLibopusEncoder(output) {
		return &ErrPrerequisite{Tool: "FFmpeg libopus encoder", Err: errMissingLibopus}
	}

	return nil
}

func hasLibopusEncoder(output []byte) bool {
	for line := range strings.SplitSeq(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[0], "A") && fields[1] == "libopus" {
			return true
		}
	}

	return false
}

func runFFmpeg(ctx context.Context, path string, args ...string) ([]byte, error) {
	commandCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	output, err := exec.CommandContext(commandCtx, path, args...).CombinedOutput() //nolint:gosec // Local FFmpeg path and fixed argument arrays.
	if err != nil {
		if commandCtx.Err() != nil {
			return nil, fmt.Errorf("FFmpeg interrupted: %w", commandCtx.Err())
		}

		return nil, fmt.Errorf("FFmpeg failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return output, nil
}

func validPrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	for _, character := range prefix {
		if character > 127 || !allowedPrefixCharacter(character) {
			return false
		}
	}

	return true
}

func allowedPrefixCharacter(character rune) bool {
	return unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_'
}
