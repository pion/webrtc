// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package oggreader

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// buildOggFile generates a valid oggfile that can
// be used for tests
func buildOggContainer() []byte {
	return []byte{
		0x4f, 0x67, 0x67, 0x53, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x8e, 0x9b, 0x20, 0xaa, 0x00, 0x00,
		0x00, 0x00, 0x61, 0xee, 0x61, 0x17, 0x01, 0x13, 0x4f, 0x70,
		0x75, 0x73, 0x48, 0x65, 0x61, 0x64, 0x01, 0x02, 0x00, 0x0f,
		0x80, 0xbb, 0x00, 0x00, 0x00, 0x00, 0x00, 0x4f, 0x67, 0x67,
		0x53, 0x00, 0x00, 0xda, 0x93, 0xc2, 0xd9, 0x00, 0x00, 0x00,
		0x00, 0x8e, 0x9b, 0x20, 0xaa, 0x02, 0x00, 0x00, 0x00, 0x49,
		0x97, 0x03, 0x37, 0x01, 0x05, 0x98, 0x36, 0xbe, 0x88, 0x9e,
	}
}

func TestOggReader_ParseValidHeader(t *testing.T) {
	reader, header, err := NewWith(bytes.NewReader(buildOggContainer()))
	assert.NoError(t, err)
	assert.NotNil(t, reader)
	assert.NotNil(t, header)

	assert.EqualValues(t, header.ChannelMap, 0)
	assert.EqualValues(t, header.Channels, 2)
	assert.EqualValues(t, header.OutputGain, 0)
	assert.EqualValues(t, header.PreSkip, 0xf00)
	assert.EqualValues(t, header.SampleRate, 48000)
	assert.EqualValues(t, header.Version, 1)
}

func TestOggReader_ParseNextPage(t *testing.T) {
	ogg := bytes.NewReader(buildOggContainer())

	reader, _, err := NewWith(ogg)
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	payload, _, err := reader.ParseNextPage()
	assert.Equal(t, []byte{0x98, 0x36, 0xbe, 0x88, 0x9e}, payload)
	assert.NoError(t, err)

	_, _, err = reader.ParseNextPage()
	assert.Equal(t, err, io.EOF)
}

func TestOggReader_ParseErrors(t *testing.T) {
	t.Run("Assert that Reader isn't nil", func(t *testing.T) {
		_, _, err := NewWith(nil)
		assert.Equal(t, err, errNilStream)
	})

	t.Run("Invalid ID Page Header Signature", func(t *testing.T) {
		ogg := buildOggContainer()
		ogg[0] = 0

		_, _, err := newWith(bytes.NewReader(ogg), false)
		assert.Equal(t, err, errBadIDPageSignature)
	})

	t.Run("Invalid ID Page Header Type", func(t *testing.T) {
		ogg := buildOggContainer()
		ogg[5] = 0

		_, _, err := newWith(bytes.NewReader(ogg), false)
		assert.Equal(t, err, errBadIDPageType)
	})

	t.Run("Invalid ID Page Payload Length", func(t *testing.T) {
		ogg := buildOggContainer()
		ogg[27] = 0

		_, _, err := newWith(bytes.NewReader(ogg), false)
		assert.Equal(t, err, errBadIDPageLength)
	})

	t.Run("Invalid ID Page Payload Length", func(t *testing.T) {
		ogg := buildOggContainer()
		ogg[35] = 0

		_, _, err := newWith(bytes.NewReader(ogg), false)
		assert.Equal(t, err, errBadIDPagePayloadSignature)
	})

	t.Run("Invalid Page Checksum", func(t *testing.T) {
		ogg := buildOggContainer()
		ogg[22] = 0

		_, _, err := NewWith(bytes.NewReader(ogg))
		assert.Equal(t, err, errChecksumMismatch)
	})
}
