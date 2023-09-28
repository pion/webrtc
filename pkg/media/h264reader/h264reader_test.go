// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package h264reader

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func CreateReader(h264 []byte, require *require.Assertions) *H264Reader {
	reader, err := NewReader(bytes.NewReader(h264))

	require.Nil(err)
	require.NotNil(reader)

	return reader
}

func TestDataDoesNotStartWithH264Header(t *testing.T) {
	require := require.New(t)

	testFunction := func(input []byte, expectedErr error) {
		reader := CreateReader(input, require)
		nal, err := reader.NextNAL()
		require.ErrorIs(err, expectedErr)
		require.Nil(nal)
	}

	h264Bytes1 := []byte{2}
	testFunction(h264Bytes1, io.EOF)

	h264Bytes2 := []byte{0, 2}
	testFunction(h264Bytes2, io.EOF)

	h264Bytes3 := []byte{0, 0, 2}
	testFunction(h264Bytes3, io.EOF)

	h264Bytes4 := []byte{0, 0, 2, 0}
	testFunction(h264Bytes4, errDataIsNotH264Stream)

	h264Bytes5 := []byte{0, 0, 0, 2}
	testFunction(h264Bytes5, errDataIsNotH264Stream)
}

func TestParseHeader(t *testing.T) {
	require := require.New(t)
	h264Bytes := []byte{0x0, 0x0, 0x1, 0xAB}

	reader := CreateReader(h264Bytes, require)

	nal, err := reader.NextNAL()
	require.Nil(err)

	require.Equal(1, len(nal.Data))
	require.True(nal.ForbiddenZeroBit)
	require.Equal(uint32(0), nal.PictureOrderCount)
	require.Equal(uint8(1), nal.RefIdc)
	require.Equal(NalUnitTypeEndOfStream, nal.UnitType)
}

func TestEOF(t *testing.T) {
	require := require.New(t)

	testFunction := func(input []byte) {
		reader := CreateReader(input, require)

		nal, err := reader.NextNAL()
		require.Equal(io.EOF, err)
		require.Nil(nal)
	}

	h264Bytes1 := []byte{0, 0, 0, 1}
	testFunction(h264Bytes1)

	h264Bytes2 := []byte{0, 0, 1}
	testFunction(h264Bytes2)

	h264Bytes3 := []byte{}
	testFunction(h264Bytes3)
}

func TestSkipSEI(t *testing.T) {
	require := require.New(t)
	h264Bytes := []byte{
		0x0, 0x0, 0x0, 0x1, 0xAA,
		0x0, 0x0, 0x0, 0x1, 0x6, // SEI
		0x0, 0x0, 0x0, 0x1, 0xAB,
	}

	reader := CreateReader(h264Bytes, require)

	nal, err := reader.NextNAL()
	require.Nil(err)
	require.Equal(byte(0xAA), nal.Data[0])

	nal, err = reader.NextNAL()
	require.Nil(err)
	require.Equal(byte(0xAB), nal.Data[0])
}

func TestIssue1734_NextNal(t *testing.T) {
	tt := [...][]byte{
		[]byte("\x00\x00\x010\x00\x00\x01\x00\x00\x01"),
		[]byte("\x00\x00\x00\x01\x00\x00\x01"),
	}

	for _, cur := range tt {
		r, err := NewReader(bytes.NewReader(cur))
		require.NoError(t, err)

		// Just make sure it doesn't crash
		for {
			nal, err := r.NextNAL()

			if err != nil || nal == nil {
				break
			}
		}
	}
}

func TestTrailing01AfterStartCode(t *testing.T) {
	r, err := NewReader(bytes.NewReader([]byte{
		0x0, 0x0, 0x0, 0x1, 0x01,
		0x0, 0x0, 0x0, 0x1, 0x01,
	}))
	require.NoError(t, err)

	for i := 0; i <= 1; i++ {
		nal, err := r.NextNAL()
		require.NoError(t, err)
		require.NotNil(t, nal)
	}
}
