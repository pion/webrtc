// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package oggwriter

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"
	"github.com/stretchr/testify/assert"
)

var (
	errUnexpectedSeek    = errors.New("unexpected seek")
	errUnexpectedWriteAt = errors.New("unexpected write-at")
	errRewriteFailed     = errors.New("rewrite failed")
	errCloseFailed       = errors.New("close failed")
)

type oggWriterPacketTest struct {
	buffer       io.Writer
	message      string
	messageClose string
	packet       *rtp.Packet
	writer       *OggWriter
	err          error
	closeErr     error
}

type noSeekWriter struct {
	bytes.Buffer
	closed        bool
	seekCalled    bool
	writeAtCalled bool
}

type rewriteFailingCloser struct {
	bytes.Buffer
	closed       bool
	writeAtCalls int
	writeAtErr   error
	closeErr     error
}

type rawOggPage struct {
	header       []byte
	segmentTable []byte
	payload      []byte
}

func (w *noSeekWriter) Seek(int64, int) (int64, error) {
	w.seekCalled = true

	return 0, errUnexpectedSeek
}

func (w *noSeekWriter) WriteAt([]byte, int64) (int, error) {
	w.writeAtCalled = true

	return 0, errUnexpectedWriteAt
}

func (w *noSeekWriter) Close() error {
	w.closed = true

	return nil
}

func (w *rewriteFailingCloser) Seek(offset int64, whence int) (int64, error) {
	if offset != 0 || whence != io.SeekCurrent {
		return 0, errUnexpectedSeek
	}

	return int64(w.Len()), nil
}

func (w *rewriteFailingCloser) WriteAt(p []byte, _ int64) (int, error) {
	w.writeAtCalls++
	if w.writeAtErr != nil {
		return 0, w.writeAtErr
	}

	return len(p), nil
}

func (w *rewriteFailingCloser) Close() error {
	w.closed = true

	return w.closeErr
}

func TestOggWriter_AddPacketAndClose(t *testing.T) {
	rawPkt := []byte{
		0x90, 0xe0, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
		0x27, 0x82, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x98, 0x36, 0xbe, 0x88, 0x9e,
	}

	validPacket := &rtp.Packet{
		Header: rtp.Header{
			Marker:           true,
			Extension:        true,
			ExtensionProfile: 1,
			Version:          2,
			PayloadType:      111,
			SequenceNumber:   27023,
			Timestamp:        3653407706,
			SSRC:             476325762,
			CSRC:             []uint32{},
		},
		Payload: rawPkt[20:],
	}
	assert.NoError(t, validPacket.SetExtension(0, []byte{0xFF, 0xFF, 0xFF, 0xFF}))

	assert := assert.New(t)

	// The linter misbehave and thinks this code is the same as the tests in ivf-writer_test
	// nolint:dupl
	addPacketTestCase := []oggWriterPacketTest{
		{
			buffer:       &bytes.Buffer{},
			message:      "OggWriter shouldn't be able to write something to a closed file",
			messageClose: "OggWriter should be able to close an already closed file",
			packet:       validPacket,
			err:          errFileNotOpened,
			closeErr:     nil,
		},
		{
			buffer:       &bytes.Buffer{},
			message:      "OggWriter shouldn't be able to write a nil packet",
			messageClose: "OggWriter should be able to close the file",
			packet:       nil,
			err:          errInvalidNilPacket,
			closeErr:     nil,
		},
		{
			buffer:       &bytes.Buffer{},
			message:      "OggWriter should be able to write an Opus packet",
			messageClose: "OggWriter should be able to close the file",
			packet:       validPacket,
			err:          nil,
			closeErr:     nil,
		},
		{
			buffer:       nil,
			message:      "OggWriter shouldn't be able to write something to a closed file",
			messageClose: "OggWriter should be able to close an already closed file",
			packet:       nil,
			err:          errFileNotOpened,
			closeErr:     nil,
		},
	}

	// First test case has a 'nil' file descriptor
	writer, err := NewWith(addPacketTestCase[0].buffer, 48000, 2)
	assert.Nil(err, "OggWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	err = writer.Close()
	assert.Nil(err, "OggWriter should be able to close the file descriptor")
	writer.stream = nil
	addPacketTestCase[0].writer = writer

	// Second test writes tries to write an empty packet
	writer, err = NewWith(addPacketTestCase[1].buffer, 48000, 2)
	assert.Nil(err, "OggWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	addPacketTestCase[1].writer = writer

	// Third test writes tries to write a valid Opus packet
	writer, err = NewWith(addPacketTestCase[2].buffer, 48000, 2)
	assert.Nil(err, "OggWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	addPacketTestCase[2].writer = writer

	// Fourth test tries to write to a nil stream
	writer, err = NewWith(addPacketTestCase[3].buffer, 4800, 2)
	assert.NotNil(err, "IVFWriter shouldn't be created")
	assert.Nil(writer, "Writer should be nil")
	addPacketTestCase[3].writer = writer

	for _, t := range addPacketTestCase {
		if t.writer != nil {
			res := t.writer.WriteRTP(t.packet)
			assert.Equal(t.err, res, t.message)
		}
	}

	for _, t := range addPacketTestCase {
		if t.writer != nil {
			res := t.writer.Close()
			assert.Equal(t.closeErr, res, t.messageClose)
		}
	}
}

func TestOggWriter_EmptyPayload(t *testing.T) {
	buffer := &bytes.Buffer{}

	writer, err := NewWith(buffer, 48000, 2)
	assert.NoError(t, err)

	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{}}))
}

func TestOggWriter_RejectsUnsupportedChannelCount(t *testing.T) {
	tests := []struct {
		name         string
		channelCount uint16
	}{
		{
			name:         "zero",
			channelCount: 0,
		},
		{
			name:         "three",
			channelCount: 3,
		},
		{
			name:         "wraps to zero",
			channelCount: 256,
		},
		{
			name:         "max uint16",
			channelCount: 65535,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName := filepath.Join(t.TempDir(), "invalid.ogg")

			writer, err := New(fileName, 48000, tt.channelCount)
			assert.ErrorIs(t, err, errInvalidChannelCount)
			assert.Nil(t, writer)
			_, statErr := os.Stat(fileName)
			assert.ErrorIs(t, statErr, os.ErrNotExist)

			writer, err = NewWith(&bytes.Buffer{}, 48000, tt.channelCount)
			assert.ErrorIs(t, err, errInvalidChannelCount)
			assert.Nil(t, writer)

			multiWriter, err := NewWriter(&bytes.Buffer{}, WithChannelCount(tt.channelCount))
			assert.ErrorIs(t, err, errInvalidChannelCount)
			assert.Nil(t, multiWriter)

			multiWriter, err = NewWriter(&bytes.Buffer{})
			assert.NoError(t, err)
			assert.NotNil(t, multiWriter)

			track, err := multiWriter.NewTrack(1111, WithChannelCount(tt.channelCount))
			assert.ErrorIs(t, err, errInvalidChannelCount)
			assert.Nil(t, track)
		})
	}
}

func TestOggWriter_NewCloseWithoutRTPPreservesHeaders(t *testing.T) {
	fileName := t.TempDir() + "/empty.ogg"

	writer, err := New(fileName, 48000, 2)
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())

	file, err := os.Open(fileName) //nolint:gosec
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, file.Close())
	}()

	reader, header, err := oggreader.NewWith(file)
	assert.NoError(t, err)
	assert.NotNil(t, reader)
	assert.NotNil(t, header)

	payload, pageHeader, err := reader.ParseNextPage()
	assert.NoError(t, err)

	headerType, ok := pageHeader.HeaderType(payload)
	assert.True(t, ok)
	assert.Equal(t, oggreader.HeaderOpusTags, headerType)
}

func TestOggWriter_LargePayload(t *testing.T) {
	rawPkt := bytes.Repeat([]byte{0x45}, 1000)

	validPacket := &rtp.Packet{
		Header: rtp.Header{
			Marker:           true,
			Extension:        true,
			ExtensionProfile: 1,
			Version:          2,
			PayloadType:      111,
			SequenceNumber:   27023,
			Timestamp:        3653407706,
			SSRC:             476325762,
			CSRC:             []uint32{},
		},
		Payload: rawPkt,
	}
	assert.NoError(t, validPacket.SetExtension(0, []byte{0xFF, 0xFF, 0xFF, 0xFF}))

	writer, err := NewWith(&bytes.Buffer{}, 48000, 2)
	assert.NoError(t, err, "OggWriter should be created")
	assert.NotNil(t, writer, "Writer shouldn't be nil")

	err = writer.WriteRTP(validPacket)
	assert.NoError(t, err)

	data := createPageForSerial(
		writer.checksumTable,
		rawPkt,
		pageHeaderTypeContinuationOfStream,
		0,
		writer.track.serial,
		1,
	)
	assert.Equal(t, uint8(4), data[26])
}

func TestOggWriter_BoundaryPayloadSplitsTerminatingLace(t *testing.T) {
	payload := bytes.Repeat([]byte{0x45}, 65025)
	track := newTrackState(48000, 2, 0x01020304, defaultOpusTags())
	track.pageIndex = 7
	buffer := &bytes.Buffer{}

	err := writePage(
		buffer,
		nil,
		generateChecksumTable(),
		track,
		payload,
		pageHeaderTypeContinuationOfStream,
		960,
	)
	assert.NoError(t, err)
	assert.Equal(t, uint32(9), track.pageIndex)

	pages := parseRawOggPages(t, buffer.Bytes())
	if !assert.Len(t, pages, 2) {
		return
	}

	assert.Equal(t, uint8(pageHeaderTypeContinuationOfStream), pages[0].header[5])
	assert.Equal(t, noGranulePosition, binary.LittleEndian.Uint64(pages[0].header[6:14]))
	assert.Equal(t, uint32(7), binary.LittleEndian.Uint32(pages[0].header[18:22]))
	assert.Equal(t, uint8(maxOggPageSegments), pages[0].header[26])
	assert.Equal(t, bytes.Repeat([]byte{255}, maxOggPageSegments), pages[0].segmentTable)
	assert.Len(t, pages[0].payload, 65025)

	assert.Equal(t, uint8(pageHeaderTypeContinuationOfPacket), pages[1].header[5])
	assert.Equal(t, uint64(960), binary.LittleEndian.Uint64(pages[1].header[6:14]))
	assert.Equal(t, uint32(8), binary.LittleEndian.Uint32(pages[1].header[18:22]))
	assert.Equal(t, uint8(1), pages[1].header[26])
	assert.Equal(t, []byte{0}, pages[1].segmentTable)
	assert.Empty(t, pages[1].payload)
}

func TestOggWriter_BoundaryPayloadCloseMarksEmptyContinuationPageEOS(t *testing.T) {
	fileName := filepath.Join(t.TempDir(), "boundary.ogg")

	writer, err := New(fileName, 48000, 2)
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	payload := bytes.Repeat([]byte{0x45}, 65025)
	assert.NoError(t, writer.WriteRTP(opusRTPPacketForTest(t, 1000, writer.track.serial, payload)))
	assert.NoError(t, writer.Close())

	data, err := os.ReadFile(fileName) //nolint:gosec
	assert.NoError(t, err)
	pages := parseRawOggPages(t, data)
	if !assert.Len(t, pages, 4) {
		return
	}

	assert.Equal(t, uint8(pageHeaderTypeContinuationOfPacket|pageHeaderTypeEndOfStream), pages[3].header[5])
	assert.Equal(t, []byte{0}, pages[3].segmentTable)
	assert.Empty(t, pages[3].payload)
}

func TestOggWriter_CloseClosesFileAfterEOSRewriteFailure(t *testing.T) {
	fileName := filepath.Join(t.TempDir(), "append.ogg")
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec
	assert.NoError(t, err)

	writer, err := newWith(file, file, 48000, 2)
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	assert.NoError(t, writer.WriteRTP(validOpusPacketForTest(t, 1000, writer.track.serial)))

	err = writer.Close()
	assert.Error(t, err)

	_, err = file.Write([]byte{0})
	assert.ErrorIs(t, err, os.ErrClosed)
}

func parseRawOggPages(t *testing.T, data []byte) []rawOggPage {
	t.Helper()

	var pages []rawOggPage
	for offset := 0; offset < len(data); {
		if offset+pageHeaderSize > len(data) {
			assert.Failf(t, "short Ogg page header", "at offset %d", offset)

			return nil
		}

		header := data[offset : offset+pageHeaderSize]
		segmentCount := int(header[26])
		segmentTableOffset := offset + pageHeaderSize
		payloadOffset := segmentTableOffset + segmentCount
		if payloadOffset > len(data) {
			assert.Failf(t, "short Ogg segment table", "at offset %d", segmentTableOffset)

			return nil
		}

		segmentTable := data[segmentTableOffset:payloadOffset]
		payloadSize := 0
		for _, segmentSize := range segmentTable {
			payloadSize += int(segmentSize)
		}

		nextOffset := payloadOffset + payloadSize
		if nextOffset > len(data) {
			assert.Failf(t, "short Ogg page payload", "at offset %d", payloadOffset)

			return nil
		}

		pages = append(pages, rawOggPage{
			header:       header,
			segmentTable: segmentTable,
			payload:      data[payloadOffset:nextOffset],
		})
		offset = nextOffset
	}

	return pages
}

func validOpusPacketForTest(t *testing.T, timestamp, ssrc uint32) *rtp.Packet {
	t.Helper()

	return opusRTPPacketForTest(t, timestamp, ssrc, []byte{0x98, 0x36, 0xbe, 0x88, 0x9e})
}

func opusRTPPacketForTest(t *testing.T, timestamp, ssrc uint32, payload []byte) *rtp.Packet {
	t.Helper()
	packet := &rtp.Packet{
		Header: rtp.Header{
			Marker:           true,
			Extension:        true,
			ExtensionProfile: 1,
			Version:          2,
			PayloadType:      111,
			SequenceNumber:   27023,
			Timestamp:        timestamp,
			SSRC:             ssrc,
			CSRC:             []uint32{},
		},
		Payload: append([]byte(nil), payload...),
	}
	assert.NoError(t, packet.SetExtension(0, []byte{0xFF, 0xFF, 0xFF, 0xFF}))

	return packet
}

func TestOggWriter_OpusPacketSampleCount(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    uint64
		wantErr error
	}{
		{
			name:    "silk 60ms",
			payload: []byte{0x18},
			want:    2880,
		},
		{
			name:    "hybrid 10ms",
			payload: []byte{0x60},
			want:    480,
		},
		{
			name:    "hybrid 20ms",
			payload: []byte{0x68},
			want:    960,
		},
		{
			name:    "celt 2.5ms",
			payload: []byte{0x80},
			want:    120,
		},
		{
			name:    "celt 20ms one frame",
			payload: []byte{0x98},
			want:    960,
		},
		{
			name:    "code 1 two frames",
			payload: []byte{0x99},
			want:    1920,
		},
		{
			name:    "code 2 two frames",
			payload: []byte{0x9a},
			want:    1920,
		},
		{
			name:    "code 3 frame count",
			payload: []byte{0x9b, 0x03},
			want:    2880,
		},
		{
			name:    "maximum packet duration",
			payload: []byte{0x1b, 0x02},
			want:    5760,
		},
		{
			name:    "missing code 3 count",
			payload: []byte{0x9b},
			wantErr: errInvalidOpusPacket,
		},
		{
			name:    "zero code 3 frame count",
			payload: []byte{0x9b, 0x00},
			wantErr: errInvalidOpusPacket,
		},
		{
			name:    "over maximum packet duration",
			payload: []byte{0x1b, 0x03},
			wantErr: errInvalidOpusPacket,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := opusPacketSampleCount(tt.payload)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOggWriter_GranulePositionUsesOpusDuration(t *testing.T) {
	buffer := &bytes.Buffer{}

	writer, err := NewWith(buffer, 48000, 2)
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	assert.NoError(t, writer.WriteRTP(opusRTPPacketForTest(t, 1000, 1111, []byte{0x98})))
	assert.NoError(t, writer.WriteRTP(opusRTPPacketForTest(t, 3880, 1111, []byte{0x98})))
	assert.NoError(t, writer.WriteRTP(opusRTPPacketForTest(t, 4840, 1111, []byte{0x9b, 0x02})))

	assert.Equal(t, []uint64{960, 1920, 3840}, audioGranulePositions(t, buffer.Bytes()))
}

func audioGranulePositions(t *testing.T, data []byte) []uint64 {
	t.Helper()

	reader, err := oggreader.NewWithOptions(bytes.NewReader(data))
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	var granules []uint64
	for {
		payload, pageHeader, parseErr := reader.ParseNextPage()
		if errors.Is(parseErr, io.EOF) {
			break
		}
		if !assert.NoError(t, parseErr) {
			break
		}

		if _, ok := pageHeader.HeaderType(payload); ok {
			continue
		}

		granules = append(granules, pageHeader.GranulePosition)
	}

	return granules
}

func opusTagsBySerial(t *testing.T, data []byte) map[uint32]*oggreader.OpusTags {
	t.Helper()

	reader, err := oggreader.NewWithOptions(bytes.NewReader(data))
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	tagsBySerial := map[uint32]*oggreader.OpusTags{}
	for {
		payload, pageHeader, parseErr := reader.ParseNextPage()
		if errors.Is(parseErr, io.EOF) {
			break
		}
		if !assert.NoError(t, parseErr) {
			break
		}

		headerType, ok := pageHeader.HeaderType(payload)
		if !ok || headerType != oggreader.HeaderOpusTags {
			continue
		}

		opusTags, err := oggreader.ParseOpusTags(payload)
		if !assert.NoError(t, err) {
			continue
		}
		tagsBySerial[pageHeader.Serial] = opusTags
	}

	return tagsBySerial
}

func TestOggWriter_NewWriterRequiresOutput(t *testing.T) {
	writer, err := NewWriter(nil)
	assert.ErrorIs(t, err, errOutputNotOpened)
	assert.Nil(t, writer)
}

func TestOggWriter_NewWriter(t *testing.T) {
	buffer := &bytes.Buffer{}
	serial1 := uint32(0x01020304)
	serial2 := uint32(0x05060708)

	writer, err := NewWriter(buffer, WithSampleRate(48000), WithChannelCount(2))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track1, err := writer.NewTrack(1111, WithSerial(serial1), WithChannelCount(1))
	assert.NoError(t, err)
	assert.NotNil(t, track1)
	track2, err := writer.NewTrack(2222, WithSerial(serial2), WithSampleRate(16000))
	assert.NoError(t, err)
	assert.NotNil(t, track2)

	assert.NoError(t, track1.WriteRTP(validOpusPacketForTest(t, 1000, 1111)))
	assert.NoError(t, track2.WriteRTP(validOpusPacketForTest(t, 2000, 2222)))

	type pageOrderEntry struct {
		kind   string
		serial uint32
	}

	reader, err := oggreader.NewWithOptions(bytes.NewReader(buffer.Bytes()))
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	var pageOrder []pageOrderEntry
	var opusHeadSerials []uint32
	var opusTagsSerials []uint32
	var audioSerials []uint32
	var audioGranules []uint64
	opusHeadSampleRates := map[uint32]uint32{}
	opusHeadChannelCounts := map[uint32]uint8{}

	for {
		payload, pageHeader, parseErr := reader.ParseNextPage()
		if errors.Is(parseErr, io.EOF) {
			break
		}
		if !assert.NoError(t, parseErr) {
			break
		}

		headerType, ok := pageHeader.HeaderType(payload)
		if ok {
			pageOrder = append(pageOrder, pageOrderEntry{
				kind:   string(headerType),
				serial: pageHeader.Serial,
			})

			switch headerType {
			case oggreader.HeaderOpusID:
				opusHeadSerials = append(opusHeadSerials, pageHeader.Serial)
				opusHeadSampleRates[pageHeader.Serial] = binary.LittleEndian.Uint32(payload[12:])
				opusHeadChannelCounts[pageHeader.Serial] = payload[9]
			case oggreader.HeaderOpusTags:
				opusTagsSerials = append(opusTagsSerials, pageHeader.Serial)
			default:
			}

			continue
		}

		pageOrder = append(pageOrder, pageOrderEntry{
			kind:   "audio",
			serial: pageHeader.Serial,
		})
		audioSerials = append(audioSerials, pageHeader.Serial)
		audioGranules = append(audioGranules, pageHeader.GranulePosition)
	}

	assert.Equal(t, []pageOrderEntry{
		{kind: string(oggreader.HeaderOpusID), serial: serial1},
		{kind: string(oggreader.HeaderOpusID), serial: serial2},
		{kind: string(oggreader.HeaderOpusTags), serial: serial1},
		{kind: string(oggreader.HeaderOpusTags), serial: serial2},
		{kind: "audio", serial: serial1},
		{kind: "audio", serial: serial2},
	}, pageOrder)
	assert.Equal(t, []uint32{serial1, serial2}, opusHeadSerials)
	assert.Equal(t, []uint32{serial1, serial2}, opusTagsSerials)
	assert.Equal(t, []uint32{serial1, serial2}, audioSerials)
	assert.Equal(t, []uint64{960, 960}, audioGranules)
	assert.Equal(t, map[uint32]uint32{
		serial1: 48000,
		serial2: 16000,
	}, opusHeadSampleRates)
	assert.Equal(t, map[uint32]uint8{
		serial1: 1,
		serial2: 2,
	}, opusHeadChannelCounts)
}

func TestOggWriter_NewWriterWritesOpusTags(t *testing.T) {
	buffer := &bytes.Buffer{}
	serial1 := uint32(0x01020304)
	serial2 := uint32(0x05060708)

	writer, err := NewWriter(
		buffer,
		WithVendor("pion-test-suite"),
		WithUserComments(UserComment{Comment: "ENCODER", Value: "pion-oggwriter"}),
	)
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track1, err := writer.NewTrack(
		1111,
		WithSerial(serial1),
		WithUserComments(
			UserComment{Comment: "ARTIST", Value: "Julia"},
			UserComment{Comment: "TITLE", Value: "First Track"},
		),
	)
	assert.NoError(t, err)
	assert.NotNil(t, track1)

	track2, err := writer.NewTrack(
		2222,
		WithSerial(serial2),
		WithVendor("track-two-vendor"),
		WithUserComments(
			UserComment{Comment: "ARTIST", Value: "Jade Bob"},
			UserComment{Comment: "ALBUM", Value: "Fanta"},
		),
	)
	assert.NoError(t, err)
	assert.NotNil(t, track2)

	assert.NoError(t, track1.WriteRTP(validOpusPacketForTest(t, 1000, 1111)))
	assert.NoError(t, track2.WriteRTP(validOpusPacketForTest(t, 2000, 2222)))

	tagsBySerial := opusTagsBySerial(t, buffer.Bytes())
	if !assert.Contains(t, tagsBySerial, serial1) || !assert.Contains(t, tagsBySerial, serial2) {
		return
	}

	assert.Equal(t, "pion-test-suite", tagsBySerial[serial1].Vendor)
	assert.Equal(t, []oggreader.UserComment{
		{Comment: "ENCODER", Value: "pion-oggwriter"},
		{Comment: "ARTIST", Value: "Julia"},
		{Comment: "TITLE", Value: "First Track"},
	}, tagsBySerial[serial1].UserComments)

	assert.Equal(t, "track-two-vendor", tagsBySerial[serial2].Vendor)
	assert.Equal(t, []oggreader.UserComment{
		{Comment: "ENCODER", Value: "pion-oggwriter"},
		{Comment: "ARTIST", Value: "Jade Bob"},
		{Comment: "ALBUM", Value: "Fanta"},
	}, tagsBySerial[serial2].UserComments)
}

func TestOggWriter_RejectsInvalidOpusTags(t *testing.T) {
	tests := []struct {
		name        string
		option      WriterTrackOption
		errContains string
	}{
		{
			name:        "invalid vendor UTF-8",
			option:      WithVendor(string([]byte{0xff})),
			errContains: "vendor is not valid UTF-8",
		},
		{
			name:        "empty user comment name",
			option:      WithUserComments(UserComment{Comment: "", Value: "Alice"}),
			errContains: "invalid user comment name",
		},
		{
			name:        "user comment name with equals",
			option:      WithUserComments(UserComment{Comment: "ART=IST", Value: "Alice"}),
			errContains: "invalid user comment name",
		},
		{
			name:        "invalid user comment value UTF-8",
			option:      WithUserComments(UserComment{Comment: "ARTIST", Value: string([]byte{0xff})}),
			errContains: "user comment value is not valid UTF-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/writer", func(t *testing.T) {
			writer, err := NewWriter(&bytes.Buffer{}, tt.option)
			assert.ErrorIs(t, err, errInvalidOpusTags)
			assert.ErrorContains(t, err, tt.errContains)
			assert.Nil(t, writer)
		})

		t.Run(tt.name+"/track", func(t *testing.T) {
			writer, err := NewWriter(&bytes.Buffer{})
			assert.NoError(t, err)
			assert.NotNil(t, writer)

			track, err := writer.NewTrack(1111, tt.option)
			assert.ErrorIs(t, err, errInvalidOpusTags)
			assert.ErrorContains(t, err, tt.errContains)
			assert.Nil(t, track)
		})
	}
}

func TestOggWriter_RejectsOpusTagsHeaderLongerThanMaxInt(t *testing.T) {
	opusTags := OpusTags{
		Vendor: "pion",
		UserComments: []UserComment{
			{Comment: "TITLE", Value: "A long enough title"},
			{Comment: "ARTIST", Value: "Pion"},
		},
	}
	headerLen := uint64(len(commentPageSignature) + 4 + len(opusTags.Vendor) + 4)
	for _, comment := range opusTags.UserComments {
		headerLen += 4 + uint64(len(comment.Comment)) + 1 + uint64(len(comment.Value))
	}

	assert.NoError(t, validateOpusTagsWithMaxHeaderLen(opusTags, headerLen))

	err := validateOpusTagsWithMaxHeaderLen(opusTags, headerLen-1)
	assert.ErrorIs(t, err, errInvalidOpusTags)
	assert.ErrorContains(t, err, "header is too long")
}

func TestOggWriter_NewWriterAcceptsZeroSerial(t *testing.T) {
	buffer := &bytes.Buffer{}

	writer, err := NewWriter(buffer, WithSampleRate(48000), WithChannelCount(2))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track, err := writer.NewTrack(1111, WithSerial(0))
	assert.NoError(t, err)
	assert.NotNil(t, track)
	assert.Equal(t, uint32(0), track.track.serial)

	assert.NoError(t, track.WriteRTP(validOpusPacketForTest(t, 1000, 1111)))

	reader, err := oggreader.NewWithOptions(bytes.NewReader(buffer.Bytes()))
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	var serials []uint32
	for {
		_, pageHeader, parseErr := reader.ParseNextPage()
		if errors.Is(parseErr, io.EOF) {
			break
		}
		if !assert.NoError(t, parseErr) {
			break
		}

		serials = append(serials, pageHeader.Serial)
	}

	assert.Equal(t, []uint32{0, 0, 0}, serials)
}

func TestOggWriter_MultiTrackRejectsDuplicateTrackSSRC(t *testing.T) {
	writer, err := NewWriter(&bytes.Buffer{}, WithSampleRate(48000), WithChannelCount(2))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track, err := writer.NewTrack(1111, WithSerial(0x01020304))
	assert.NoError(t, err)
	assert.NotNil(t, track)

	track, err = writer.NewTrack(1111, WithSerial(0x05060708))
	assert.ErrorIs(t, err, errDuplicateTrackSSRC)
	assert.Nil(t, track)
}

func TestOggWriter_MultiTrackRejectsPacketSSRCMismatch(t *testing.T) {
	buffer := &bytes.Buffer{}

	writer, err := NewWriter(buffer, WithSampleRate(48000), WithChannelCount(2))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track, err := writer.NewTrack(1111, WithSerial(0x01020304))
	assert.NoError(t, err)
	assert.NotNil(t, track)

	err = track.WriteRTP(validOpusPacketForTest(t, 1000, 2222))
	assert.ErrorIs(t, err, errPacketSSRCMismatch)
	assert.False(t, writer.started)
	assert.Empty(t, buffer.Bytes())

	track, err = writer.NewTrack(2222, WithSerial(0x05060708))
	assert.NoError(t, err)
	assert.NotNil(t, track)
}

func TestOggWriter_MultiTrackDoesNotStartUntilPacketWillBeWritten(t *testing.T) {
	tests := []struct {
		name    string
		packet  *rtp.Packet
		wantErr error
	}{
		{
			name:    "nil packet",
			packet:  nil,
			wantErr: errInvalidNilPacket,
		},
		{
			name: "nil payload",
			packet: &rtp.Packet{
				Header: rtp.Header{SSRC: 1111},
			},
			wantErr: nil,
		},
		{
			name: "empty payload",
			packet: &rtp.Packet{
				Header:  rtp.Header{SSRC: 1111},
				Payload: []byte{},
			},
			wantErr: nil,
		},
		{
			name:    "wrong SSRC",
			packet:  validOpusPacketForTest(t, 1000, 2222),
			wantErr: errPacketSSRCMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}

			writer, err := NewWriter(buffer, WithSampleRate(48000), WithChannelCount(2))
			assert.NoError(t, err)
			assert.NotNil(t, writer)

			track1, err := writer.NewTrack(1111, WithSerial(0x01020304))
			assert.NoError(t, err)
			assert.NotNil(t, track1)

			err = track1.WriteRTP(tt.packet)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.wantErr)
			}
			assert.False(t, writer.started)
			assert.Empty(t, buffer.Bytes())

			track2, err := writer.NewTrack(2222, WithSerial(0x05060708))
			assert.NoError(t, err)
			assert.NotNil(t, track2)

			assert.NoError(t, track2.WriteRTP(validOpusPacketForTest(t, 2000, 2222)))
			assert.True(t, writer.started)
			assert.NotEmpty(t, buffer.Bytes())
		})
	}
}

func TestOggWriter_MultiTrackRejectsDuplicateTrackSerial(t *testing.T) {
	writer, err := NewWriter(&bytes.Buffer{}, WithSampleRate(48000), WithChannelCount(2))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track, err := writer.NewTrack(1111, WithSerial(0x01020304))
	assert.NoError(t, err)
	assert.NotNil(t, track)

	track, err = writer.NewTrack(2222, WithSerial(0x01020304))
	assert.ErrorIs(t, err, errDuplicateTrackSerial)
	assert.Nil(t, track)
}

func TestOggWriter_MultiTrackRejectsDuplicateZeroTrackSerial(t *testing.T) {
	writer, err := NewWriter(&bytes.Buffer{}, WithSampleRate(48000), WithChannelCount(2))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track, err := writer.NewTrack(1111, WithSerial(0))
	assert.NoError(t, err)
	assert.NotNil(t, track)

	track, err = writer.NewTrack(2222, WithSerial(0))
	assert.ErrorIs(t, err, errDuplicateTrackSerial)
	assert.Nil(t, track)
}

func TestOggWriter_MultiTrackRejectsTrackCreationAfterWrite(t *testing.T) {
	writer, err := NewWriter(&bytes.Buffer{}, WithSampleRate(48000), WithChannelCount(2))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track1, err := writer.NewTrack(1111, WithSerial(0x01020304))
	assert.NoError(t, err)
	assert.NotNil(t, track1)

	assert.NoError(t, track1.WriteRTP(validOpusPacketForTest(t, 1000, 1111)))

	track2, err := writer.NewTrack(2222, WithSerial(0x05060708))
	assert.ErrorIs(t, err, errTracksStarted)
	assert.Nil(t, track2)
}

func TestOggWriter_NewWriterDoesNotSeekWithoutOption(t *testing.T) {
	output := &noSeekWriter{}

	writer, err := NewWriter(output, WithSampleRate(48000), WithChannelCount(2))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track, err := writer.NewTrack(1111, WithSerial(0x01020304))
	assert.NoError(t, err)
	assert.NotNil(t, track)

	assert.NoError(t, track.WriteRTP(validOpusPacketForTest(t, 1000, 1111)))
	assert.NoError(t, writer.Close())
	assert.True(t, output.closed)
	assert.False(t, output.seekCalled)
	assert.False(t, output.writeAtCalled)
}

func TestOggWriter_NewWriterFinalizesUnseekableOutputWithEOS(t *testing.T) {
	output := &noSeekWriter{}
	serial1 := uint32(0x01020304)
	serial2 := uint32(0x05060708)

	writer, err := NewWriter(output, WithSampleRate(48000), WithChannelCount(2))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track1, err := writer.NewTrack(1111, WithSerial(serial1))
	assert.NoError(t, err)
	assert.NotNil(t, track1)
	track2, err := writer.NewTrack(2222, WithSerial(serial2))
	assert.NoError(t, err)
	assert.NotNil(t, track2)

	assert.NoError(t, track1.WriteRTP(validOpusPacketForTest(t, 1000, 1111)))
	assert.NoError(t, track2.WriteRTP(validOpusPacketForTest(t, 2000, 2222)))
	assert.NoError(t, writer.Close())

	pages := parseRawOggPages(t, output.Bytes())
	if !assert.Len(t, pages, 8) {
		return
	}

	eosPages := map[uint32]rawOggPage{}
	for _, page := range pages {
		if page.header[5]&pageHeaderTypeEndOfStream != 0 {
			eosPages[binary.LittleEndian.Uint32(page.header[14:18])] = page
		}
	}

	if !assert.Len(t, eosPages, 2) {
		return
	}
	for _, serial := range []uint32{serial1, serial2} {
		page := eosPages[serial]
		assert.Equal(t, uint8(pageHeaderTypeEndOfStream), page.header[5])
		assert.Equal(t, uint64(960), binary.LittleEndian.Uint64(page.header[6:14]))
		assert.Empty(t, page.segmentTable)
		assert.Empty(t, page.payload)
	}
	assert.True(t, output.closed)
	assert.False(t, output.seekCalled)
	assert.False(t, output.writeAtCalled)
}

func TestOggWriter_NewWriterCloseWithoutRTPFinalizesUnseekableOutputWithEOS(t *testing.T) {
	buffer := &bytes.Buffer{}
	serial := uint32(0x01020304)

	writer, err := NewWriter(buffer, WithSampleRate(48000), WithChannelCount(2))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track, err := writer.NewTrack(1111, WithSerial(serial))
	assert.NoError(t, err)
	assert.NotNil(t, track)

	assert.NoError(t, writer.Close())

	pages := parseRawOggPages(t, buffer.Bytes())
	if !assert.Len(t, pages, 3) {
		return
	}

	assert.Equal(t, uint8(pageHeaderTypeBeginningOfStream), pages[0].header[5])
	assert.Equal(t, uint8(pageHeaderTypeContinuationOfStream), pages[1].header[5])
	assert.Equal(t, uint8(pageHeaderTypeEndOfStream), pages[2].header[5])
	assert.Equal(t, serial, binary.LittleEndian.Uint32(pages[2].header[14:18]))
	assert.Equal(t, uint32(2), binary.LittleEndian.Uint32(pages[2].header[18:22]))
	assert.Zero(t, binary.LittleEndian.Uint64(pages[2].header[6:14]))
	assert.Empty(t, pages[2].segmentTable)
	assert.Empty(t, pages[2].payload)
}

func TestOggWriter_NewWriterCloseClosesOutputAfterStartFailure(t *testing.T) {
	output := &noSeekWriter{}

	writer, err := NewWriter(output, WithSampleRate(48000), WithChannelCount(2), WithSeekableOutput(output))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track, err := writer.NewTrack(1111, WithSerial(0x01020304))
	assert.NoError(t, err)
	assert.NotNil(t, track)

	err = writer.Close()
	assert.ErrorIs(t, err, errUnexpectedSeek)
	assert.True(t, output.closed)
	assert.True(t, output.seekCalled)
}

func TestOggWriter_NewWriterCloseClosesOutputAfterEOSRewriteFailure(t *testing.T) {
	output := &rewriteFailingCloser{
		writeAtErr: errRewriteFailed,
		closeErr:   errCloseFailed,
	}

	writer, err := NewWriter(output, WithSampleRate(48000), WithChannelCount(2), WithSeekableOutput(output))
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track1, err := writer.NewTrack(1111, WithSerial(0x01020304))
	assert.NoError(t, err)
	assert.NotNil(t, track1)
	track2, err := writer.NewTrack(2222, WithSerial(0x05060708))
	assert.NoError(t, err)
	assert.NotNil(t, track2)

	assert.NoError(t, track1.WriteRTP(validOpusPacketForTest(t, 1000, 1111)))
	assert.NoError(t, track2.WriteRTP(validOpusPacketForTest(t, 2000, 2222)))

	err = writer.Close()
	assert.ErrorIs(t, err, errRewriteFailed)
	assert.ErrorIs(t, err, errCloseFailed)
	assert.Equal(t, 2, output.writeAtCalls)
	assert.True(t, output.closed)
}

func TestOggWriter_NewWithPreservesLegacyStreamMode(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "oggwriter-*.ogg")
	assert.NoError(t, err)

	writer, err := NewWith(file, 48000, 2)
	assert.NoError(t, err)
	assert.NotNil(t, writer)
	assert.Nil(t, writer.fd)

	assert.NoError(t, writer.Close())
}

func TestOggWriter_NewWithDoesNotRetainLastPayload(t *testing.T) {
	buffer := &bytes.Buffer{}

	writer, err := NewWith(buffer, 48000, 2)
	assert.NoError(t, err)
	assert.NotNil(t, writer)
	assert.Nil(t, writer.track.lastPayload)

	assert.NoError(t, writer.WriteRTP(validOpusPacketForTest(t, 1000, writer.track.serial)))
	assert.Nil(t, writer.track.lastPayload)
	assert.Zero(t, writer.track.lastGranulePosition)
	assert.Zero(t, writer.track.lastPageIndex)
	assert.Zero(t, writer.track.lastPageOffset)
}
