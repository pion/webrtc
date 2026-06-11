// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package oggwriter

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"
	"github.com/stretchr/testify/assert"
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

func validOpusPacketForTest(t *testing.T, timestamp, ssrc uint32) *rtp.Packet {
	t.Helper()
	rawPkt := []byte{
		0x90, 0xe0, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
		0x27, 0x82, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x98, 0x36, 0xbe, 0x88, 0x9e,
	}

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
		Payload: rawPkt[20:],
	}
	assert.NoError(t, packet.SetExtension(0, []byte{0xFF, 0xFF, 0xFF, 0xFF}))

	return packet
}

func TestOggWriter_NewWriterWith(t *testing.T) {
	buffer := &bytes.Buffer{}
	serial1 := uint32(0x01020304)
	serial2 := uint32(0x05060708)

	writer, err := NewWriterWith(buffer, 48000, 2)
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

	reader, err := oggreader.NewWithOptions(bytes.NewReader(buffer.Bytes()))
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	var opusHeadSerials []uint32
	var opusTagsSerials []uint32
	var audioSerials []uint32
	var audioGranules []uint64

	for {
		payload, pageHeader, parseErr := reader.ParseNextPage()
		if errors.Is(parseErr, io.EOF) {
			break
		}
		assert.NoError(t, parseErr)

		headerType, ok := pageHeader.HeaderType(payload)
		if ok {
			switch headerType {
			case oggreader.HeaderOpusID:
				opusHeadSerials = append(opusHeadSerials, pageHeader.Serial)
			case oggreader.HeaderOpusTags:
				opusTagsSerials = append(opusTagsSerials, pageHeader.Serial)
			default:
			}

			continue
		}

		audioSerials = append(audioSerials, pageHeader.Serial)
		audioGranules = append(audioGranules, pageHeader.GranulePosition)
	}

	assert.Equal(t, []uint32{serial1, serial2}, opusHeadSerials)
	assert.Equal(t, []uint32{serial1, serial2}, opusTagsSerials)
	assert.Equal(t, []uint32{serial1, serial2}, audioSerials)
	assert.Equal(t, []uint64{1, 1}, audioGranules)
}

func TestOggWriter_MultiTrackRejectsDuplicateTrackSSRC(t *testing.T) {
	writer, err := NewWriterWith(&bytes.Buffer{}, 48000, 2)
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track, err := writer.NewTrack(1111, WithSerial(0x01020304))
	assert.NoError(t, err)
	assert.NotNil(t, track)

	track, err = writer.NewTrack(1111, WithSerial(0x05060708))
	assert.ErrorIs(t, err, errDuplicateTrackSSRC)
	assert.Nil(t, track)
}

func TestOggWriter_MultiTrackRejectsDuplicateTrackSerial(t *testing.T) {
	writer, err := NewWriterWith(&bytes.Buffer{}, 48000, 2)
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	track, err := writer.NewTrack(1111, WithSerial(0x01020304))
	assert.NoError(t, err)
	assert.NotNil(t, track)

	track, err = writer.NewTrack(2222, WithSerial(0x01020304))
	assert.ErrorIs(t, err, errDuplicateTrackSerial)
	assert.Nil(t, track)
}

func TestOggWriter_MultiTrackRejectsTrackCreationAfterWrite(t *testing.T) {
	writer, err := NewWriterWith(&bytes.Buffer{}, 48000, 2)
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

func TestOggWriter_NewWithPreservesLegacyStreamMode(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "oggwriter-*.ogg")
	assert.NoError(t, err)

	writer, err := NewWith(file, 48000, 2)
	assert.NoError(t, err)
	assert.NotNil(t, writer)
	assert.Nil(t, writer.fd)

	assert.NoError(t, writer.Close())
}
