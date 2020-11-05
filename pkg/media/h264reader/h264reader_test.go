package h264reader

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func CreateReader(h264 []byte, assert *assert.Assertions) *H264Reader {
	reader, err := NewReader(bytes.NewReader(h264))

	assert.Nil(err)
	assert.NotNil(reader)

	return reader
}

func TestDataDoesNotStartWithH264Header(t *testing.T) {
	assert := assert.New(t)

	testFunction := func(input []byte) {
		reader := CreateReader(input, assert)
		nal, err := reader.NextNAL()
		assert.Equal(errDataIsNotH264Stream, err)
		assert.Nil(nal)
	}

	h264Bytes1 := []byte{2}
	testFunction(h264Bytes1)

	h264Bytes2 := []byte{0, 2}
	testFunction(h264Bytes2)

	h264Bytes3 := []byte{0, 0, 2}
	testFunction(h264Bytes3)

	h264Bytes4 := []byte{0, 0, 2, 0}
	testFunction(h264Bytes4)

	h264Bytes5 := []byte{0, 0, 0, 2}
	testFunction(h264Bytes5)
}

func TestParseHeader(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0x0, 0x0, 0x1, 0xAB}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Nil(err)

	assert.Equal(1, len(nal.Data))
	assert.True(nal.ForbiddenZeroBit)
	assert.Equal(uint32(0), nal.PictureOrderCount)
	assert.Equal(uint8(1), nal.RefIdc)
	assert.Equal(NalUnitTypeEndOfStream, nal.UnitType)
}

func TestEOF(t *testing.T) {
	assert := assert.New(t)

	testFunction := func(input []byte) {
		reader := CreateReader(input, assert)

		nal, err := reader.NextNAL()
		assert.Equal(io.EOF, err)
		assert.Nil(nal)
	}

	h264Bytes1 := []byte{0, 0, 0, 1}
	testFunction(h264Bytes1)

	h264Bytes2 := []byte{0, 0, 1}
	testFunction(h264Bytes2)

	h264Bytes3 := []byte{}
	testFunction(h264Bytes3)
}

func TestSkipSEI(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{
		0x0, 0x0, 0x0, 0x1, 0xAA,
		0x0, 0x0, 0x0, 0x1, 0x6, // SEI
		0x0, 0x0, 0x0, 0x1, 0xAB,
	}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Nil(err)
	assert.Equal(byte(0xAA), nal.Data[0])

	nal, err = reader.NextNAL()
	assert.Nil(err)
	assert.Equal(byte(0xAB), nal.Data[0])
}
