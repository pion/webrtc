// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package ivfreader

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// buildIVFContainer takes frames and prepends valid IVF file header
func buildIVFContainer(frames ...*[]byte) *bytes.Buffer {
	// Valid IVF file header taken from: https://github.com/webmproject/...
	// vp8-test-vectors/blob/master/vp80-00-comprehensive-001.ivf
	// Video Image Width      	- 176
	// Video Image Height    	- 144
	// Frame Rate Rate        	- 30000
	// Frame Rate Scale       	- 1000
	// Video Length in Frames	- 29
	// BitRate: 		 64.01 kb/s
	ivf := []byte{
		0x44, 0x4b, 0x49, 0x46, 0x00, 0x00, 0x20, 0x00,
		0x56, 0x50, 0x38, 0x30, 0xb0, 0x00, 0x90, 0x00,
		0x30, 0x75, 0x00, 0x00, 0xe8, 0x03, 0x00, 0x00,
		0x1d, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	for f := range frames {
		ivf = append(ivf, *frames[f]...)
	}

	return bytes.NewBuffer(ivf)
}

func TestIVFReader_ParseValidFileHeader(t *testing.T) {
	assert := assert.New(t)
	ivf := buildIVFContainer(&[]byte{})

	reader, header, err := NewWith(ivf)
	assert.Nil(err, "IVFReader should be created")
	assert.NotNil(reader, "Reader shouldn't be nil")
	assert.NotNil(header, "Header shouldn't be nil")

	assert.Equal("DKIF", header.signature, "signature is 'DKIF'")
	assert.Equal(uint16(0), header.version, "version should be 0")
	assert.Equal("VP80", header.FourCC, "FourCC should be 'VP80'")
	assert.Equal(uint16(176), header.Width, "width should be 176")
	assert.Equal(uint16(144), header.Height, "height should be 144")
	assert.Equal(uint32(30000), header.TimebaseDenominator, "timebase denominator should be 30000")
	assert.Equal(uint32(1000), header.TimebaseNumerator, "timebase numerator should be 1000")
	assert.Equal(uint32(29), header.NumFrames, "number of frames should be 29")
	assert.Equal(uint32(0), header.unused, "bytes should be unused")
}

func TestIVFReader_ParseValidFrames(t *testing.T) {
	assert := assert.New(t)

	// Frame Length - 4
	// Timestamp - None
	// Frame Payload - 0xDEADBEEF
	validFrame1 := []byte{
		0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0xDE, 0xAD, 0xBE, 0xEF,
	}

	// Frame Length - 12
	// Timestamp - None
	// Frame Payload - 0xDEADBEEFDEADBEEF
	validFrame2 := []byte{
		0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0xDE, 0xAD, 0xBE, 0xEF,
		0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF,
	}

	ivf := buildIVFContainer(&validFrame1, &validFrame2)
	reader, _, err := NewWith(ivf)
	assert.Nil(err, "IVFReader should be created")
	assert.NotNil(reader, "Reader shouldn't be nil")

	// Parse Frame #1
	payload, header, err := reader.ParseNextFrame()

	assert.Nil(err, "Should have parsed frame #1 without error")
	assert.Equal(uint32(4), header.FrameSize, "Frame header frameSize should be 4")
	assert.Equal(4, len(payload), "Payload should be length 4")
	assert.Equal(
		payload,
		[]byte{
			0xDE, 0xAD, 0xBE, 0xEF,
		},
		"Payload value should be 0xDEADBEEF")
	assert.Equal(int64(ivfFrameHeaderSize+ivfFileHeaderSize+header.FrameSize), reader.bytesReadSuccesfully)
	previousBytesRead := reader.bytesReadSuccesfully

	// Parse Frame #2
	payload, header, err = reader.ParseNextFrame()

	assert.Nil(err, "Should have parsed frame #2 without error")
	assert.Equal(uint32(12), header.FrameSize, "Frame header frameSize should be 4")
	assert.Equal(12, len(payload), "Payload should be length 12")
	assert.Equal(
		payload,
		[]byte{
			0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD,
			0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF,
		},
		"Payload value should be 0xDEADBEEFDEADBEEF")
	assert.Equal(int64(ivfFrameHeaderSize+header.FrameSize)+previousBytesRead, reader.bytesReadSuccesfully)
}

func TestIVFReader_ParseIncompleteFrameHeader(t *testing.T) {
	assert := assert.New(t)

	// frame with 11-byte header (missing 1 byte)
	incompleteFrame := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00,
	}

	ivf := buildIVFContainer(&incompleteFrame)
	reader, _, err := NewWith(ivf)
	assert.Nil(err, "IVFReader should be created")
	assert.NotNil(reader, "Reader shouldn't be nil")

	// Parse Frame #1
	payload, header, err := reader.ParseNextFrame()

	assert.Nil(payload, "Payload should be nil")
	assert.Nil(header, "Incomplete header should be nil")
	assert.Equal(errIncompleteFrameHeader, err)
}

func TestIVFReader_ParseIncompleteFramePayload(t *testing.T) {
	assert := assert.New(t)

	// frame with header defining frameSize of 4
	// but only 2 bytes available (missing 2 bytes)
	incompleteFrame := []byte{
		0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0xDE, 0xAD,
	}

	ivf := buildIVFContainer(&incompleteFrame)
	reader, _, err := NewWith(ivf)
	assert.Nil(err, "IVFReader should be created")
	assert.NotNil(reader, "Reader shouldn't be nil")

	// Parse Frame #1
	payload, header, err := reader.ParseNextFrame()

	assert.Nil(payload, "Incomplete payload should be nil")
	assert.Nil(header, "Header should be nil")
	assert.Equal(errIncompleteFrameData, err)
}

func TestIVFReader_EOFWhenNoFramesLeft(t *testing.T) {
	assert := assert.New(t)

	ivf := buildIVFContainer(&[]byte{})
	reader, _, err := NewWith(ivf)
	assert.Nil(err, "IVFReader should be created")
	assert.NotNil(reader, "Reader shouldn't be nil")

	_, _, err = reader.ParseNextFrame()

	assert.Equal(io.EOF, err)
}
