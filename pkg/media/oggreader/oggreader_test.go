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
// be used for tests.
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

// buildSurroundOggContainerShort has mapping family 1 but omits the mapping table (invalid length).
func buildSurroundOggContainerShort() []byte {
	return []byte{
		0x4f, 0x67, 0x67, 0x53, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x58, 0x49, 0xac, 0xe2, 0x00, 0x00,
		0x00, 0x00, 0xc1, 0xa8, 0x7d, 0x4e, 0x01, 0x13, 0x4f, 0x70,
		0x75, 0x73, 0x48, 0x65, 0x61, 0x64, 0x01, 0x06, 0x38, 0x01,
		0x80, 0xbb, 0x00, 0x00, 0x00, 0x00, 0x01,
	}
}

// buildUnknownMappingFamilyContainer creates an ID page with an unrecognized channel mapping family.
func buildUnknownMappingFamilyContainer(mappingFamily, channels uint8) []byte {
	payload := []byte{
		0x4f, 0x70, 0x75, 0x73, 0x48, 0x65, 0x61, 0x64, // "OpusHead"
		0x01,       // version
		channels,   // channel count
		0x38, 0x01, // preskip (0x0138)
		0x80, 0xbb, 0x00, 0x00, // sample rate (48000)
		0x00, 0x00, // output gain
		mappingFamily,
	}

	segmentTable := []byte{byte(len(payload))}

	header := []byte{
		0x4f, 0x67, 0x67, 0x53, // "OggS"
		0x00,                                           // version
		0x02,                                           // beginning of stream
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // granule position
		0x00, 0x00, 0x00, 0x00, // bitstream serial number
		0x00, 0x00, 0x00, 0x00, // page sequence number
		0x00, 0x00, 0x00, 0x00, // checksum (ignored with checksum disabled)
		0x01, // page segments
	}

	packet := make([]byte, 0, len(header)+len(segmentTable)+len(payload))
	packet = append(packet, header...)
	packet = append(packet, segmentTable...)
	packet = append(packet, payload...)

	return packet
}

// buildChannelMappingFamilyContainer builds an Opus ID page for mapping families that
// follow the Figure 3 layout (families 1, 2, 3, 255).
func buildChannelMappingFamilyContainer(
	mappingFamily, channels, streamCount, coupledCount uint8,
	mapping []byte,
) []byte {
	payload := []byte{
		0x4f, 0x70, 0x75, 0x73, 0x48, 0x65, 0x61, 0x64, // "OpusHead"
		0x01,       // version
		channels,   // channel count
		0x38, 0x01, // preskip (0x0138)
		0x80, 0xbb, 0x00, 0x00, // sample rate (48000)
		0x00, 0x00, // output gain
		mappingFamily,
		streamCount,
		coupledCount,
	}
	payload = append(payload, mapping...)

	segmentTable := []byte{byte(len(payload))}

	header := []byte{
		0x4f, 0x67, 0x67, 0x53, // "OggS"
		0x00,                                           // version
		0x02,                                           // header type (beginning of stream)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // granule position
		0x00, 0x00, 0x00, 0x00, // bitstream serial number
		0x00, 0x00, 0x00, 0x00, // page sequence number
		0x00, 0x00, 0x00, 0x00, // checksum (ignored when checksum disabled)
		0x01, // page segments
	}

	packet := make([]byte, 0, len(header)+len(segmentTable)+len(payload))
	packet = append(packet, header...)
	packet = append(packet, segmentTable...)
	packet = append(packet, payload...)

	return packet
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
	assert.Equal(t, int64(47), reader.bytesReadSuccesfully)

	payload, _, err := reader.ParseNextPage()
	assert.Equal(t, []byte{0x98, 0x36, 0xbe, 0x88, 0x9e}, payload)
	assert.NoError(t, err)
	assert.Equal(t, int64(80), reader.bytesReadSuccesfully)

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

	t.Run("Invalid Multichannel ID Page Payload Length", func(t *testing.T) {
		_, _, err := newWith(bytes.NewReader(buildSurroundOggContainerShort()), false)
		assert.Equal(t, err, errBadIDPageLength)
	})

	t.Run("Unsupported Channel Mapping Family", func(t *testing.T) {
		_, _, err := newWith(bytes.NewReader(buildUnknownMappingFamilyContainer(4, 2)), false)
		assert.Equal(t, err, errUnsupportedChannelMappingFamily)
	})
}

func TestOggReader_ChannelMappingFamily1(t *testing.T) {
	type testCase struct {
		name       string
		channels   uint8
		streams    uint8
		coupled    uint8
		channelMap []byte
	}

	cases := []testCase{
		{name: "1-mono", channels: 1, streams: 1, coupled: 0, channelMap: []byte{0}},
		{name: "2-stereo", channels: 2, streams: 1, coupled: 1, channelMap: []byte{0, 1}},
		{name: "3-linear-surround", channels: 3, streams: 2, coupled: 1, channelMap: []byte{0, 2, 1}},
		{name: "4-quad", channels: 4, streams: 2, coupled: 2, channelMap: []byte{0, 1, 2, 3}},
		{name: "5-5.0", channels: 5, streams: 3, coupled: 2, channelMap: []byte{0, 1, 2, 3, 4}},
		{name: "6-5.1", channels: 6, streams: 4, coupled: 2, channelMap: []byte{0, 4, 1, 2, 3, 5}},
		{name: "7-6.1", channels: 7, streams: 4, coupled: 3, channelMap: []byte{0, 1, 2, 3, 4, 5, 6}},
		{name: "8-7.1", channels: 8, streams: 5, coupled: 3, channelMap: []byte{0, 1, 2, 3, 4, 5, 6, 7}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			reader, header, err := newWith(bytes.NewReader(
				buildChannelMappingFamilyContainer(1, tc.channels, tc.streams, tc.coupled, tc.channelMap),
			), false)
			assert.NoError(t, err)
			assert.NotNil(t, reader)
			assert.NotNil(t, header)

			assert.EqualValues(t, 1, header.Version)
			assert.EqualValues(t, tc.channels, header.Channels)
			assert.EqualValues(t, 0x138, header.PreSkip)
			assert.EqualValues(t, 48e3, header.SampleRate)
			assert.EqualValues(t, 0, header.OutputGain)
			assert.EqualValues(t, 1, header.ChannelMap)
			assert.EqualValues(t, tc.streams, header.StreamCount)
			assert.EqualValues(t, tc.coupled, header.CoupledCount)
			assert.Equal(t, string(tc.channelMap), header.ChannelMapping)
		})
	}
}

func TestOggReader_KnownChannelMappingFamilies(t *testing.T) {
	cases := []struct {
		name          string
		mappingFamily uint8
		channels      uint8
		streams       uint8
		coupled       uint8
		channelMap    []byte
	}{
		{name: "family-2", mappingFamily: 2, channels: 2, streams: 1, coupled: 1, channelMap: []byte{0, 1}},
		{name: "family-255", mappingFamily: 255, channels: 2, streams: 1, coupled: 1, channelMap: []byte{0, 1}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			reader, header, err := newWith(bytes.NewReader(
				buildChannelMappingFamilyContainer(tc.mappingFamily, tc.channels, tc.streams, tc.coupled, tc.channelMap),
			), false)
			assert.NoError(t, err)
			assert.NotNil(t, reader)
			assert.NotNil(t, header)

			assert.EqualValues(t, tc.mappingFamily, header.ChannelMap)
			assert.EqualValues(t, tc.channels, header.Channels)
			assert.EqualValues(t, 0x138, header.PreSkip)
			assert.EqualValues(t, 48e3, header.SampleRate)
			assert.EqualValues(t, 0, header.OutputGain)
		})
	}
}

func TestOggReader_ParseExtraFieldsForNonZeroMappingFamily(t *testing.T) {
	cases := []struct {
		name          string
		mappingFamily uint8
		channels      uint8
		streams       uint8
		coupled       uint8
		channelMap    []byte
	}{
		{name: "family-1-stereo", mappingFamily: 1, channels: 2, streams: 1, coupled: 1, channelMap: []byte{0, 1}},
		{name: "family-1-5.1", mappingFamily: 1, channels: 6, streams: 4, coupled: 2, channelMap: []byte{0, 4, 1, 2, 3, 5}},
		{
			name:          "family-1-7.1",
			mappingFamily: 1,
			channels:      8,
			streams:       5,
			coupled:       3,
			channelMap:    []byte{0, 1, 2, 3, 4, 5, 6, 7},
		},
		{name: "family-2", mappingFamily: 2, channels: 4, streams: 2, coupled: 2, channelMap: []byte{0, 1, 2, 3}},
		{name: "family-255", mappingFamily: 255, channels: 5, streams: 3, coupled: 2, channelMap: []byte{0, 1, 2, 3, 4}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			reader, header, err := newWith(bytes.NewReader(
				buildChannelMappingFamilyContainer(tc.mappingFamily, tc.channels, tc.streams, tc.coupled, tc.channelMap),
			), false)
			assert.NoError(t, err)
			assert.NotNil(t, reader)
			assert.NotNil(t, header)

			assert.EqualValues(t, tc.mappingFamily, header.ChannelMap)
			assert.EqualValues(t, tc.channels, header.Channels)
			assert.EqualValues(t, tc.streams, header.StreamCount)
			assert.EqualValues(t, tc.coupled, header.CoupledCount)
			assert.Equal(t, string(tc.channelMap), header.ChannelMapping)
		})
	}
}
