package h264reader

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func CreateReader(h264 []byte, assert *assert.Assertions) *H264Reader {
	reader, err := NewReader(bytes.NewReader(h264))

	assert.Nil(err)
	assert.NotNil(reader)

	return reader
}

func TestNoData(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{}

	reader := CreateReader(h264Bytes, assert)

	_, err := reader.NextNAL()
	assert.NotNil(err)
}

func TestDataDoesNotStartWithH264Header1(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{2}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Equal(errDataIsNotH264Stream, err)
	assert.Nil(nal)
}

func TestDataDoesNotStartWithH264Header2(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0, 2}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Equal(errDataIsNotH264Stream, err)
	assert.Nil(nal)
}

func TestDataDoesNotStartWithH264Header3(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0, 0, 2}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Equal(errDataIsNotH264Stream, err)
	assert.Nil(nal)
}

func TestDataDoesNotStartWithH264Header4(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0, 0, 2, 0}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Equal(errDataIsNotH264Stream, err)
	assert.Nil(nal)
}

func TestDataDoesNotStartWithH264Header5(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0, 0, 0, 2}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Equal(errDataIsNotH264Stream, err)
	assert.Nil(nal)
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

func TestNotEnoughData1(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0, 0, 0, 1}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Equal(errNotEnoughData, err)
	assert.Nil(nal)
}

func TestNotEnoughData2(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0, 0, 1}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Equal(errNotEnoughData, err)
	assert.Nil(nal)
}

func TestTwoPrefixes1(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0x0, 0x0, 0x1, 0xAB, 0x0, 0x0, 0x1}
	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Nil(err)
	assert.Equal(1, len(nal.Data))
}

func TestTwoPrefixes3(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0x0, 0x0, 0x0, 0x1, 0xAB, 0x0, 0x0, 0x0, 0x01}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Nil(err)
	assert.Equal(1, len(nal.Data))
}

func TestStreamEnd1(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0x0, 0x0, 0x0, 0x1, 0xAB}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Nil(err)
	assert.Equal(1, len(nal.Data))

	nal, err = reader.NextNAL()
	assert.Equal(errNotEnoughData, err)
	assert.Nil(nal)
}

func TestStreamEnd2(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0x0, 0x0, 0x0, 0x1, 0xAB, 0x0, 0x0, 0x1}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Nil(err)
	assert.Equal(1, len(nal.Data))

	nal, err = reader.NextNAL()
	assert.Equal(errNotEnoughData, err)
	assert.Nil(nal)
}

func Test2NALs(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{0x0, 0x0, 0x1, 0xAA, 0x0, 0x0, 0x1, 0xAB}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Nil(err)
	assert.Equal(1, len(nal.Data))

	nal, err = reader.NextNAL()
	assert.Nil(err)
	assert.Equal(1, len(nal.Data))
}

func TestSkipSEI(t *testing.T) {
	assert := assert.New(t)
	h264Bytes := []byte{
		0x0, 0x0, 0x1, 0xAA,
		0x0, 0x0, 0x1, 0x6,
		0x0, 0x0, 0x1, 0xAB,
	}

	reader := CreateReader(h264Bytes, assert)

	nal, err := reader.NextNAL()
	assert.Nil(err)
	assert.Equal(byte(0xAA), nal.Data[0])

	nal, err = reader.NextNAL()
	assert.Nil(err)
	assert.Equal(byte(0xAB), nal.Data[0])
}
