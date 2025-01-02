// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package ivfwriter

import (
	"bytes"
	"io"
	"testing"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/stretchr/testify/assert"
)

type ivfWriterPacketTest struct {
	buffer       io.Writer
	message      string
	messageClose string
	packet       *rtp.Packet
	writer       *IVFWriter
	err          error
	closeErr     error
}

func TestIVFWriter_Basic(t *testing.T) {
	assert := assert.New(t)
	addPacketTestCase := []ivfWriterPacketTest{
		{
			buffer:       &bytes.Buffer{},
			message:      "IVFWriter shouldn't be able to write something to a closed file",
			messageClose: "IVFWriter should be able to close an already closed file",
			packet:       nil,
			err:          errFileNotOpened,
			closeErr:     nil,
		},
		{
			buffer:       &bytes.Buffer{},
			message:      "IVFWriter shouldn't be able to write something an empty packet",
			messageClose: "IVFWriter should be able to close the file",
			packet:       &rtp.Packet{},
			err:          errInvalidNilPacket,
			closeErr:     nil,
		},
		{
			buffer:       nil,
			message:      "IVFWriter shouldn't be able to write something to a closed file",
			messageClose: "IVFWriter should be able to close an already closed file",
			packet:       nil,
			err:          errFileNotOpened,
			closeErr:     nil,
		},
	}

	// First test case has a 'nil' file descriptor
	writer, err := NewWith(addPacketTestCase[0].buffer)
	assert.Nil(err, "IVFWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	assert.False(writer.seenKeyFrame, "Writer's seenKeyFrame should initialize false")
	assert.Equal(uint64(0), writer.count, "Writer's packet count should initialize 0")
	err = writer.Close()
	assert.Nil(err, "IVFWriter should be able to close the stream")
	writer.ioWriter = nil
	addPacketTestCase[0].writer = writer

	// Second test tries to write an empty packet
	writer, err = NewWith(addPacketTestCase[1].buffer)
	assert.Nil(err, "IVFWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	assert.False(writer.seenKeyFrame, "Writer's seenKeyFrame should initialize false")
	assert.Equal(uint64(0), writer.count, "Writer's packet count should initialize 0")
	addPacketTestCase[1].writer = writer

	// Fourth test tries to write to a nil stream
	writer, err = NewWith(addPacketTestCase[2].buffer)
	assert.NotNil(err, "IVFWriter shouldn't be created")
	assert.Nil(writer, "Writer should be nil")
	addPacketTestCase[2].writer = writer
}

func TestIVFWriter_VP8(t *testing.T) {
	// Construct valid packet
	rawValidPkt := []byte{
		0x90, 0xe0, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
		0x27, 0x82, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x98, 0x36, 0xbe, 0x89, 0x9e,
	}

	validPacket := &rtp.Packet{
		Header: rtp.Header{
			Marker:           true,
			Extension:        true,
			ExtensionProfile: 1,
			Version:          2,
			PayloadType:      96,
			SequenceNumber:   27023,
			Timestamp:        3653407706,
			SSRC:             476325762,
			CSRC:             []uint32{},
		},
		Payload: rawValidPkt[20:],
	}
	assert.NoError(t, validPacket.SetExtension(0, []byte{0xFF, 0xFF, 0xFF, 0xFF}))

	// Construct mid partition packet
	rawMidPartPkt := []byte{
		0x90, 0xe0, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
		0x27, 0x82, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x88, 0x36, 0xbe, 0x89, 0x9e,
	}

	midPartPacket := &rtp.Packet{
		Header: rtp.Header{
			Marker:           true,
			Extension:        true,
			ExtensionProfile: 1,
			Version:          2,
			PayloadType:      96,
			SequenceNumber:   27023,
			Timestamp:        3653407706,
			SSRC:             476325762,
			CSRC:             []uint32{},
		},
		Payload: rawMidPartPkt[20:],
	}
	assert.NoError(t, midPartPacket.SetExtension(0, []byte{0xFF, 0xFF, 0xFF, 0xFF}))

	// Construct keyframe packet
	rawKeyframePkt := []byte{
		0x90, 0xe0, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
		0x27, 0x82, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x98, 0x36, 0xbe, 0x88, 0x9e,
	}

	keyframePacket := &rtp.Packet{
		Header: rtp.Header{
			Marker:           true,
			Extension:        true,
			ExtensionProfile: 1,
			Version:          2,
			PayloadType:      96,
			SequenceNumber:   27023,
			Timestamp:        3653407706,
			SSRC:             476325762,
			CSRC:             []uint32{},
		},
		Payload: rawKeyframePkt[20:],
	}
	assert.NoError(t, keyframePacket.SetExtension(0, []byte{0xFF, 0xFF, 0xFF, 0xFF}))

	assert := assert.New(t)

	// Check valid packet parameters
	vp8Packet := codecs.VP8Packet{}
	_, err := vp8Packet.Unmarshal(validPacket.Payload)
	assert.Nil(err, "Packet did not process")
	assert.Equal(uint8(1), vp8Packet.S, "Start packet S value should be 1")
	assert.Equal(uint8(1), vp8Packet.Payload[0]&0x01, "Non Keyframe packet P value should be 1")

	// Check mid partition packet parameters
	vp8Packet = codecs.VP8Packet{}
	_, err = vp8Packet.Unmarshal(midPartPacket.Payload)
	assert.Nil(err, "Packet did not process")
	assert.Equal(uint8(0), vp8Packet.S, "Mid Partition packet S value should be 0")
	assert.Equal(uint8(1), vp8Packet.Payload[0]&0x01, "Non Keyframe packet P value should be 1")

	// Check keyframe packet parameters
	vp8Packet = codecs.VP8Packet{}
	_, err = vp8Packet.Unmarshal(keyframePacket.Payload)
	assert.Nil(err, "Packet did not process")
	assert.Equal(uint8(1), vp8Packet.S, "Start packet S value should be 1")
	assert.Equal(uint8(0), vp8Packet.Payload[0]&0x01, "Keyframe packet P value should be 0")

	// The linter misbehave and thinks this code is the same as the tests in oggwriter_test
	// nolint:dupl
	addPacketTestCase := []ivfWriterPacketTest{
		{
			buffer:       &bytes.Buffer{},
			message:      "IVFWriter should be able to write an IVF packet",
			messageClose: "IVFWriter should be able to close the file",
			packet:       validPacket,
			err:          nil,
			closeErr:     nil,
		},
		{
			buffer:       &bytes.Buffer{},
			message:      "IVFWriter should be able to write a Keframe IVF packet",
			messageClose: "IVFWriter should be able to close the file",
			packet:       keyframePacket,
			err:          nil,
			closeErr:     nil,
		},
	}

	// first test tries to write a valid VP8 packet
	writer, err := NewWith(addPacketTestCase[0].buffer, WithCodec(mimeTypeVP8))
	assert.Nil(err, "IVFWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	assert.False(writer.seenKeyFrame, "Writer's seenKeyFrame should initialize false")
	assert.Equal(uint64(0), writer.count, "Writer's packet count should initialize 0")
	addPacketTestCase[0].writer = writer

	// second test tries to write a keyframe packet
	writer, err = NewWith(addPacketTestCase[1].buffer)
	assert.Nil(err, "IVFWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	assert.False(writer.seenKeyFrame, "Writer's seenKeyFrame should initialize false")
	assert.Equal(uint64(0), writer.count, "Writer's packet count should initialize 0")
	addPacketTestCase[1].writer = writer

	for _, t := range addPacketTestCase {
		if t.writer != nil {
			res := t.writer.WriteRTP(t.packet)
			assert.Equal(res, t.err, t.message)
		}
	}

	// Third test tries to write a valid VP8 packet - No Keyframe
	assert.False(addPacketTestCase[0].writer.seenKeyFrame, "Writer's seenKeyFrame should remain false")
	assert.Equal(uint64(0), addPacketTestCase[0].writer.count, "Writer's packet count should remain 0")
	assert.Equal(nil, addPacketTestCase[0].writer.WriteRTP(midPartPacket), "Write packet failed") // add a mid partition packet
	assert.Equal(uint64(0), addPacketTestCase[0].writer.count, "Writer's packet count should remain 0")

	// Fifth test tries to write a keyframe packet
	assert.True(addPacketTestCase[1].writer.seenKeyFrame, "Writer's seenKeyFrame should now be true")
	assert.Equal(uint64(1), addPacketTestCase[1].writer.count, "Writer's packet count should now be 1")
	assert.Equal(nil, addPacketTestCase[1].writer.WriteRTP(midPartPacket), "Write packet failed") // add a mid partition packet
	assert.Equal(uint64(1), addPacketTestCase[1].writer.count, "Writer's packet count should remain 1")
	assert.Equal(nil, addPacketTestCase[1].writer.WriteRTP(validPacket), "Write packet failed") // add a valid packet
	assert.Equal(uint64(2), addPacketTestCase[1].writer.count, "Writer's packet count should now be 2")

	for _, t := range addPacketTestCase {
		if t.writer != nil {
			res := t.writer.Close()
			assert.Equal(res, t.closeErr, t.messageClose)
		}
	}
}

func TestIVFWriter_EmptyPayload(t *testing.T) {
	buffer := &bytes.Buffer{}

	writer, err := NewWith(buffer)
	assert.NoError(t, err)

	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{}}))
}

func TestIVFWriter_Errors(t *testing.T) {
	// Creating a Writer with AV1 and VP8
	_, err := NewWith(&bytes.Buffer{}, WithCodec(mimeTypeAV1), WithCodec(mimeTypeAV1))
	assert.ErrorIs(t, err, errCodecAlreadySet)

	// Creating a Writer with Invalid Codec
	_, err = NewWith(&bytes.Buffer{}, WithCodec(""))
	assert.ErrorIs(t, err, errNoSuchCodec)
}

func TestIVFWriter_AV1(t *testing.T) {
	t.Run("Unfragmented", func(t *testing.T) {
		buffer := &bytes.Buffer{}

		writer, err := NewWith(buffer, WithCodec(mimeTypeAV1))
		assert.NoError(t, err)

		assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0x00, 0x01, 0xFF}}))
		assert.NoError(t, writer.Close())
		assert.Equal(t, buffer.Bytes(), []byte{
			0x44, 0x4b, 0x49, 0x46, 0x0, 0x0, 0x20,
			0x0, 0x41, 0x56, 0x30, 0x31, 0x80, 0x2,
			0xe0, 0x1, 0x1e, 0x0, 0x0, 0x0, 0x1, 0x0,
			0x0, 0x0, 0x84, 0x3, 0x0, 0x0, 0x0, 0x0,
			0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
			0x0, 0x0, 0x0, 0x0, 0x0, 0xff,
		})
	})

	t.Run("Fragmented", func(t *testing.T) {
		buffer := &bytes.Buffer{}

		writer, err := NewWith(buffer, WithCodec(mimeTypeAV1))
		assert.NoError(t, err)

		for _, p := range [][]byte{{0x40, 0x02, 0x00, 0x01}, {0xc0, 0x02, 0x02, 0x03}, {0xc0, 0x02, 0x04, 0x04}} {
			assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: p}))
			assert.Equal(t, buffer.Bytes(), []byte{
				0x44, 0x4b, 0x49, 0x46, 0x0,
				0x0, 0x20, 0x0, 0x41, 0x56, 0x30,
				0x31, 0x80, 0x2, 0xe0, 0x1, 0x1e,
				0x0, 0x0, 0x0, 0x1, 0x0, 0x0,
				0x0, 0x84, 0x3, 0x0, 0x0, 0x0, 0x0,
				0x0, 0x0,
			})
		}
		assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0x80, 0x01, 0x05}}))
		assert.Equal(t, buffer.Bytes(), []byte{
			0x44, 0x4b, 0x49, 0x46, 0x0, 0x0, 0x20, 0x0, 0x41, 0x56, 0x30, 0x31, 0x80,
			0x2, 0xe0, 0x1, 0x1e, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x84, 0x3, 0x0, 0x0,
			0x0, 0x0, 0x0, 0x0, 0x7, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
			0x0, 0x1, 0x2, 0x3, 0x4, 0x4, 0x5,
		})
		assert.NoError(t, writer.Close())
	})
}
