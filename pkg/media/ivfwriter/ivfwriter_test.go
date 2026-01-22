// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
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
	// add a mid partition packet
	assert.Equal(nil, addPacketTestCase[0].writer.WriteRTP(midPartPacket), "Write packet failed")
	assert.Equal(uint64(0), addPacketTestCase[0].writer.count, "Writer's packet count should remain 0")

	// Fifth test tries to write a keyframe packet
	assert.True(addPacketTestCase[1].writer.seenKeyFrame, "Writer's seenKeyFrame should now be true")
	assert.Equal(uint64(1), addPacketTestCase[1].writer.count, "Writer's packet count should now be 1")
	// add a mid partition packet
	assert.Equal(nil, addPacketTestCase[1].writer.WriteRTP(midPartPacket), "Write packet failed")
	assert.Equal(uint64(1), addPacketTestCase[1].writer.count, "Writer's packet count should remain 1")
	// add a valid packet
	assert.Equal(nil, addPacketTestCase[1].writer.WriteRTP(validPacket), "Write packet failed")
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

		assert.NoError(
			t,
			writer.WriteRTP(
				&rtp.Packet{
					Header: rtp.Header{Marker: true},
					// N = 1, Length = 1, OBU_TYPE = 4
					Payload: []byte{0x08, 0x01, 0x20},
				}),
		)

		assert.NoError(t, writer.Close())
		assert.Equal(t, buffer.Bytes(), []byte{
			0x44, 0x4b, 0x49, 0x46, 0x0, 0x0, 0x20, 0x0, 0x41, 0x56, 0x30, 0x31,
			0x80, 0x2, 0xe0, 0x1, 0x1e, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x84,
			0x3, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x4, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
			0x0, 0x0, 0x0, 0x0, 0x0, 0x12, 0x0, 0x22, 0x0,
		})
	})

	t.Run("Fragmented", func(t *testing.T) {
		buffer := &bytes.Buffer{}

		writer, err := NewWith(buffer, WithCodec(mimeTypeAV1))
		assert.NoError(t, err)

		for _, p := range [][]byte{
			{0x48, 0x02, 0x00, 0x01}, // Y=true
			{0xc0, 0x02, 0x02, 0x03}, // Z=true, Y=true
			{0xc0, 0x02, 0x04, 0x04}, // Z=true, Y=true
			{0x80, 0x01, 0x05},       // Z=true, Y=false (But we still don't set Marker to true)
		} {
			assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: p, Header: rtp.Header{Marker: false}}))
			assert.Equal(t, buffer.Bytes(), []byte{
				0x44, 0x4b, 0x49, 0x46, 0x0,
				0x0, 0x20, 0x0, 0x41, 0x56, 0x30,
				0x31, 0x80, 0x2, 0xe0, 0x1, 0x1e,
				0x0, 0x0, 0x0, 0x1, 0x0, 0x0,
				0x0, 0x84, 0x3, 0x0, 0x0, 0x0, 0x0,
				0x0, 0x0,
			})
		}
		assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0x08, 0x01, 0x20}, Header: rtp.Header{Marker: true}}))
		assert.Equal(t, buffer.Bytes(), []byte{
			0x44, 0x4b, 0x49, 0x46, 0x0, 0x0, 0x20, 0x0, 0x41, 0x56, 0x30, 0x31, 0x80, 0x2, 0xe0, 0x1, 0x1e,
			0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x84, 0x3, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xc, 0x0, 0x0, 0x0,
			0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x12, 0x0, 0x2, 0x6, 0x1, 0x2, 0x3, 0x4, 0x4, 0x5, 0x22, 0x0,
		})
		assert.NoError(t, writer.Close())
	})

	t.Run("Invalid OBU", func(t *testing.T) {
		buffer := &bytes.Buffer{}

		writer, err := NewWith(buffer, WithCodec(mimeTypeAV1))
		assert.NoError(t, err)

		assert.Error(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0x08, 0x02, 0xff}}))
		assert.Error(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0x08, 0x01, 0xff}}))
	})

	t.Run("Skips middle sequence start", func(t *testing.T) {
		buffer := &bytes.Buffer{}

		writer, err := NewWith(buffer, WithCodec(mimeTypeAV1))
		assert.NoError(t, err)

		assert.NoError(t, writer.WriteRTP(&rtp.Packet{Header: rtp.Header{Marker: true}, Payload: []byte{0x00, 0x01, 0x20}}))

		assert.NoError(
			t,
			writer.WriteRTP(
				&rtp.Packet{
					Header: rtp.Header{Marker: true},
					// N = 1, Length = 1, OBU_TYPE = 4
					Payload: []byte{0x08, 0x01, 0x20},
				},
			),
		)

		assert.NoError(t, writer.Close())
		assert.Equal(t, buffer.Bytes(), []byte{
			0x44, 0x4b, 0x49, 0x46, 0x0, 0x0, 0x20, 0x0, 0x41, 0x56, 0x30, 0x31,
			0x80, 0x2, 0xe0, 0x1, 0x1e, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x84,
			0x3, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x4, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
			0x0, 0x0, 0x0, 0x0, 0x0, 0x12, 0x0, 0x22, 0x0,
		})
	})
}

func TestIVFWriter_VP9(t *testing.T) {
	buffer := &bytes.Buffer{}
	writer, err := NewWith(buffer, WithCodec(mimeTypeVP9))
	assert.NoError(t, err)

	// No keyframe yet, ignore non-keyframe packets (P)
	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0xD0, 0x02, 0xAA}}))
	assert.Equal(t, buffer.Bytes(), []byte{
		0x44, 0x4b, 0x49, 0x46, 0x00, 0x00, 0x20, 0x00, 0x56, 0x50, 0x39, 0x30, 0x80, 0x02, 0xe0, 0x01,
		0x1e, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x84, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	})

	// No current frame, ignore packets that don't start a frame (B)
	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0x00, 0xAA}}))
	assert.Equal(t, buffer.Bytes(), []byte{
		0x44, 0x4b, 0x49, 0x46, 0x00, 0x00, 0x20, 0x00, 0x56, 0x50, 0x39, 0x30, 0x80, 0x02, 0xe0, 0x01,
		0x1e, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x84, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	})

	// B packet, no marker bit
	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0x08, 0xAA}}))
	assert.Equal(t, buffer.Bytes(), []byte{
		0x44, 0x4b, 0x49, 0x46, 0x00, 0x00, 0x20, 0x00, 0x56, 0x50, 0x39, 0x30, 0x80, 0x02, 0xe0, 0x01,
		0x1e, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x84, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	})

	// B packet, Marker Bit
	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Header: rtp.Header{Marker: true}, Payload: []byte{0x08, 0xAB}}))
	assert.Equal(t, buffer.Bytes(), []byte{
		0x44, 0x4b, 0x49, 0x46, 0x00, 0x00, 0x20, 0x00, 0x56, 0x50, 0x39, 0x30, 0x80, 0x02, 0xe0, 0x01,
		0x1e, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x84, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xaa, 0xab,
	})
}

func TestIVFWriter_WithWidthAndHeight(t *testing.T) {
	buffer := &bytes.Buffer{}

	writer, err := NewWith(buffer, WithWidthAndHeight(789, 652))
	assert.NoError(t, err)

	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{}}))
	assert.NoError(t, writer.Close())

	assert.Equal(t, []byte{
		0x44, 0x4b, 0x49, 0x46, 0x00, 0x00, 0x20, 0x00, 0x56, 0x50, 0x38, 0x30, 0x15, 0x03, 0x8c, 0x02,
		0x1e, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x84, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}, buffer.Bytes())
}

func TestIVFWriter_WithFrameRate(t *testing.T) {
	buffer := &bytes.Buffer{}

	writer, err := NewWith(buffer, WithFrameRate(60, 1))
	assert.NoError(t, err)

	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{}}))
	assert.NoError(t, writer.Close())

	assert.Equal(t, []byte{
		0x44, 0x4b, 0x49, 0x46, 0x00, 0x00, 0x20, 0x00, 0x56, 0x50, 0x38, 0x30, 0x80, 0x02, 0xe0, 0x01,
		0x01, 0x00, 0x00, 0x00, 0x3c, 0x00, 0x00, 0x00, 0x84, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}, buffer.Bytes())
}

func TestIVFWriter_WithDirectPTS(t *testing.T) {
	buffer := &bytes.Buffer{}

	writer, err := NewWith(buffer, WithFrameRate(1, 90000), WithDirectPTS())
	assert.NoError(t, err)
	assert.True(t, writer.directPTS)
	assert.Equal(t, uint32(1), writer.timebaseNumerator)
	assert.Equal(t, uint32(90000), writer.timebaseDenominator)

	assert.NoError(t, writer.Close())
}

func TestIVFWriter_DirectPTS_VP8(t *testing.T) {
	buffer := &bytes.Buffer{}

	writer, err := NewWith(buffer, WithCodec(mimeTypeVP8), WithFrameRate(1, 90000), WithDirectPTS())
	assert.NoError(t, err)

	// Write keyframe with timestamp 0.
	keyframePacket := &rtp.Packet{
		Header: rtp.Header{
			Marker:    true,
			Timestamp: 0,
		},
		// VP8 keyframe: S=1, P=0
		Payload: []byte{0x10, 0x00, 0x00, 0x9d, 0x01, 0x2a},
	}
	assert.NoError(t, writer.WriteRTP(keyframePacket))
	assert.Equal(t, uint64(1), writer.count)

	// Write second frame with timestamp 6000 (15fps at 90kHz clock).
	frame2 := &rtp.Packet{
		Header: rtp.Header{
			Marker:    true,
			Timestamp: 6000,
		},
		// VP8 interframe: S=1, P=1
		Payload: []byte{0x10, 0x01, 0x00, 0x9d, 0x01, 0x2a},
	}
	assert.NoError(t, writer.WriteRTP(frame2))
	assert.Equal(t, uint64(2), writer.count)

	// Write third frame with timestamp 12000.
	frame3 := &rtp.Packet{
		Header: rtp.Header{
			Marker:    true,
			Timestamp: 12000,
		},
		Payload: []byte{0x10, 0x01, 0x00, 0x9d, 0x01, 0x2a},
	}
	assert.NoError(t, writer.WriteRTP(frame3))
	assert.Equal(t, uint64(3), writer.count)

	assert.NoError(t, writer.Close())

	// Verify IVF structure.
	data := buffer.Bytes()
	assert.True(t, len(data) > 32, "Buffer should contain header + frames")

	// Check IVF header timebase (offset 16-20: denominator, offset 20-24: numerator).
	timebaseDenom := uint32(data[16]) | uint32(data[17])<<8 | uint32(data[18])<<16 | uint32(data[19])<<24
	timebaseNum := uint32(data[20]) | uint32(data[21])<<8 | uint32(data[22])<<16 | uint32(data[23])<<24
	assert.Equal(t, uint32(90000), timebaseDenom)
	assert.Equal(t, uint32(1), timebaseNum)

	// Verify PTS values in frame headers.
	// Frame 1: PTS should be 0.
	pts1 := uint64(data[36]) | uint64(data[37])<<8 | uint64(data[38])<<16 | uint64(data[39])<<24 |
		uint64(data[40])<<32 | uint64(data[41])<<40 | uint64(data[42])<<48 | uint64(data[43])<<56
	assert.Equal(t, uint64(0), pts1, "First frame PTS should be 0")

	// Frame 2: PTS should be 6000 (RTP timestamp directly).
	frameSize1 := uint32(data[32]) | uint32(data[33])<<8 | uint32(data[34])<<16 | uint32(data[35])<<24
	frame2Offset := 32 + 12 + int(frameSize1)
	pts2 := uint64(data[frame2Offset+4]) | uint64(data[frame2Offset+5])<<8 |
		uint64(data[frame2Offset+6])<<16 | uint64(data[frame2Offset+7])<<24 |
		uint64(data[frame2Offset+8])<<32 | uint64(data[frame2Offset+9])<<40 |
		uint64(data[frame2Offset+10])<<48 | uint64(data[frame2Offset+11])<<56
	assert.Equal(t, uint64(6000), pts2, "Second frame PTS should be 6000")
}

func TestIVFWriter_DirectPTS_Precision(t *testing.T) {
	buffer := &bytes.Buffer{}

	writer, err := NewWith(buffer, WithCodec(mimeTypeVP8), WithFrameRate(1, 90000), WithDirectPTS())
	assert.NoError(t, err)

	// Simulate 15fps video (6000 RTP ticks per frame at 90kHz).
	// 225 frames = 15 seconds.
	timestamps := make([]uint32, 225)
	for idx := range timestamps {
		timestamps[idx] = uint32(idx) * 6000 //nolint:gosec // Test code with known safe values.
	}

	for idx, ts := range timestamps {
		packet := &rtp.Packet{
			Header: rtp.Header{
				Marker:    true,
				Timestamp: ts,
			},
			// VP8 keyframe for first, interframe for rest.
			Payload: []byte{0x10, 0x00, 0x00, 0x9d, 0x01, 0x2a},
		}
		if idx > 0 {
			packet.Payload[1] = 0x01 // Set non-keyframe flag.
		}
		assert.NoError(t, writer.WriteRTP(packet))
	}

	assert.NoError(t, writer.Close())

	// Verify frame count.
	assert.Equal(t, uint64(225), writer.count)

	// Verify last frame PTS is exactly 224 * 6000 = 1344000.
	data := buffer.Bytes()
	offset := 32 // Start after IVF header.

	var lastPTS uint64
	for idx := 0; idx < 225; idx++ {
		frameSize := uint32(data[offset]) | uint32(data[offset+1])<<8 |
			uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24
		lastPTS = uint64(data[offset+4]) | uint64(data[offset+5])<<8 |
			uint64(data[offset+6])<<16 | uint64(data[offset+7])<<24 |
			uint64(data[offset+8])<<32 | uint64(data[offset+9])<<40 |
			uint64(data[offset+10])<<48 | uint64(data[offset+11])<<56
		offset += 12 + int(frameSize)
	}

	// Last frame should have PTS = 224 * 6000 = 1344000.
	assert.Equal(t, uint64(224*6000), lastPTS, "Last frame PTS should be exactly 1344000")
}

func TestIVFWriter_BackwardCompatibility(t *testing.T) {
	// Test that default behavior (without WithDirectPTS) remains unchanged.
	buffer := &bytes.Buffer{}

	writer, err := NewWith(buffer, WithCodec(mimeTypeVP8))
	assert.NoError(t, err)
	assert.False(t, writer.directPTS, "Default should not use direct PTS mode")

	// Write keyframe.
	keyframePacket := &rtp.Packet{
		Header: rtp.Header{
			Marker:    true,
			Timestamp: 90000, // 1 second at 90kHz.
		},
		Payload: []byte{0x10, 0x00, 0x00, 0x9d, 0x01, 0x2a},
	}
	assert.NoError(t, writer.WriteRTP(keyframePacket))

	// Write second frame at 2 seconds.
	frame2 := &rtp.Packet{
		Header: rtp.Header{
			Marker:    true,
			Timestamp: 180000, // 2 seconds at 90kHz.
		},
		Payload: []byte{0x10, 0x01, 0x00, 0x9d, 0x01, 0x2a},
	}
	assert.NoError(t, writer.WriteRTP(frame2))
	assert.NoError(t, writer.Close())

	// Verify PTS uses millisecond conversion (legacy behavior).
	data := buffer.Bytes()

	// First frame PTS should be 0.
	pts1 := uint64(data[36]) | uint64(data[37])<<8 | uint64(data[38])<<16 | uint64(data[39])<<24 |
		uint64(data[40])<<32 | uint64(data[41])<<40 | uint64(data[42])<<48 | uint64(data[43])<<56
	assert.Equal(t, uint64(0), pts1)

	// Second frame: (180000-90000)/90000 * 1000 = 1000ms, then 1000 * 1 / 30 = 33 PTS.
	frameSize1 := uint32(data[32]) | uint32(data[33])<<8 | uint32(data[34])<<16 | uint32(data[35])<<24
	frame2Offset := 32 + 12 + int(frameSize1)
	pts2 := uint64(data[frame2Offset+4]) | uint64(data[frame2Offset+5])<<8 |
		uint64(data[frame2Offset+6])<<16 | uint64(data[frame2Offset+7])<<24 |
		uint64(data[frame2Offset+8])<<32 | uint64(data[frame2Offset+9])<<40 |
		uint64(data[frame2Offset+10])<<48 | uint64(data[frame2Offset+11])<<56
	assert.Equal(t, uint64(33), pts2, "Legacy mode: PTS should be 33 (1000ms * 1/30)")
}
